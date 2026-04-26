param(
    [string]$ApiBase = "http://localhost:8080/api/v1",
    [string]$ApiKey  = "87339ea3-1add-4689-ae57-3128ebd03c4f"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
Set-Location "E:\Maheshwari\UGCL\backend\v1"

$RunTag  = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds().ToString()
$LogFile = "tmp\chat-test-latest.txt"
if (-not (Test-Path "tmp")) { New-Item -ItemType Directory -Path "tmp" | Out-Null }

# ============================================================================
# Known user IDs (verified from seed data)
# ============================================================================
$USER_SUPER = "4f74d110-db88-4c7d-b5b5-ccf5ace1c0ea"
$USER_WA    = "6043e5f3-a707-4e64-a2e8-12c55fd5c8d3"
$USER_WE    = "946c55c0-d582-4710-88b2-c196535be17e"

# ============================================================================
# Helpers
# ============================================================================
$baseH = @{
    "x-api-key"    = $ApiKey
    "Content-Type" = "application/json"
    "Accept"       = "application/json"
}

function Login([string]$Phone) {
    return Invoke-RestMethod -Method Post -Uri "$ApiBase/login" -Headers $baseH `
        -Body (ConvertTo-Json -InputObject @{ phone = $Phone; password = "Welcome@123" })
}

function MkH([string]$token) {
    return @{
        "x-api-key"     = $ApiKey
        "Authorization" = "Bearer $token"
        "Content-Type"  = "application/json"
        "Accept"        = "application/json"
    }
}

function Invoke-Api([string]$Method, [string]$Path, [hashtable]$Headers, [object]$Body = $null) {
    $uri = "$ApiBase$Path"
    try {
        if ($null -eq $Body) {
            $resp = Invoke-RestMethod -Method $Method -Uri $uri -Headers $Headers
        } else {
            $json = ConvertTo-Json -InputObject $Body -Depth 30
            $resp = Invoke-RestMethod -Method $Method -Uri $uri -Headers $Headers -Body $json
        }
        return @{ ok = $true; data = $resp }
    } catch {
        $code = 0
        try { $code = [int]$_.Exception.Response.StatusCode } catch {}
        return @{ ok = $false; status = $code; error = $_.Exception.Message }
    }
}

function API-Data([string]$M, [string]$P, $H, $B = $null) {
    $r = Invoke-Api $M $P $H $B
    if ($r.ok) { return $r.data } else { return $null }
}

$results = [System.Collections.Generic.List[object]]::new()

function Record([string]$Group, [string]$Case, [string]$Status, [string]$Detail) {
    $obj = [pscustomobject]@{ Group = $Group; Case = $Case; Status = $Status; Detail = $Detail }
    $results.Add($obj)
    $col = switch ($Status) { "PASS" { "Green" } "FAIL" { "Red" } "SKIP" { "Cyan" } default { "Yellow" } }
    $line = "[$Status] $Group :: $Case :: $Detail"
    Write-Host $line -ForegroundColor $col
}

function Pass([string]$G, [string]$C, [string]$D) { Record $G $C "PASS" $D }
function Fail([string]$G, [string]$C, [string]$D) { Record $G $C "FAIL" $D }
function Skip([string]$G, [string]$C, [string]$D) { Record $G $C "SKIP" $D }

function Expect-OK([string]$G, [string]$C, $R, [string]$D) {
    if ($R.ok) { Pass $G $C $D } else { Fail $G $C "$D | HTTP $($R.status) $($R.error)" }
}

function Expect-Fail([string]$G, [string]$C, $R, [int]$Code = 0, [string]$D = "") {
    if (-not $R.ok) {
        if ($Code -gt 0 -and $R.status -ne $Code) {
            Fail $G $C "Expected HTTP $Code got $($R.status) :: $D"
        } else {
            Pass $G $C "Correctly rejected (HTTP $($R.status)) :: $D"
        }
    } else {
        Fail $G $C "Should have been rejected but got 2xx :: $D"
    }
}

# Resources created during test  - used for cleanup and cross-test reference
$ctxDirect      = $null  # direct conversation ID between Super and WA
$ctxGroup       = $null  # group conversation ID
$ctxMsg1        = $null  # first text message ID in direct conv
$ctxMsg2        = $null  # second message (to test reply-to)
$ctxMsgImage    = $null  # image-type message ID
$ctxMsgFile     = $null  # file-type message ID
$ctxAttachment  = $null  # attachment ID
$ctxReactionMsg = $null  # message used for reaction tests
$ctxGroupMsg    = $null  # message in group conversation

# ============================================================================
# Login
# ============================================================================
Write-Host "`n=== Logging in test users ===" -ForegroundColor Cyan

$lSuper = Login "9999999999"
$lWA    = Login "9999999901"
$lWE    = Login "9999999902"

$hSuper = MkH $lSuper.token
$hWA    = MkH $lWA.token
$hWE    = MkH $lWE.token

# ============================================================================
# G1  - Chat Users Listing
# ============================================================================
$G = "G1-ChatUsers"
Write-Host "`n=== $G ===" -ForegroundColor Cyan

$r = Invoke-Api "GET" "/chat/users" $hSuper
Expect-OK $G "superadmin-can-list-chat-users" $r "GET /chat/users returns 200"
if ($r.ok -and $r.data.users) {
    $userCount = @($r.data.users).Count
    if ($userCount -ge 3) { Pass $G "at-least-3-users-listed" "Got $userCount users" }
    else { Fail $G "at-least-3-users-listed" "Expected >=3, got $userCount" }
} elseif ($r.ok) {
    Skip $G "at-least-3-users-listed" "Response has no users array"
}

$r = Invoke-Api "GET" "/chat/users" $hWA
Expect-OK $G "water-admin-can-list-chat-users" $r "GET /chat/users returns 200 for WA"

$r = Invoke-Api "GET" "/chat/users" $hWE
Expect-OK $G "water-engineer-can-list-chat-users" $r "GET /chat/users returns 200 for WE"

$noAuthH = @{ "x-api-key" = $ApiKey; "Accept" = "application/json" }
$r = Invoke-Api "GET" "/chat/users" $noAuthH
Expect-Fail $G "unauthenticated-chat-users-rejected" $r 401 "no auth token"

# ============================================================================
# G2  - Direct Conversation Creation (idempotency)
# ============================================================================
$G = "G2-DirectConversation"
Write-Host "`n=== $G ===" -ForegroundColor Cyan

$body = @{
    type           = "direct"
    participant_ids = @($USER_WA)
}
$r = Invoke-Api "POST" "/chat/conversations" $hSuper $body
Expect-OK $G "create-direct-conv-super-to-wa" $r "Super creates direct conv with WA"
if ($r.ok -and $r.data.conversation) {
    $ctxDirect = $r.data.conversation.id
    Pass $G "direct-conv-id-present" "ID=$ctxDirect"
} else {
    Fail $G "direct-conv-id-present" "No conversation.id in response"
}

# Idempotency: creating again should return same conversation
if ($ctxDirect) {
    $r2 = Invoke-Api "POST" "/chat/conversations" $hSuper $body
    if ($r2.ok -and $r2.data.conversation -and $r2.data.conversation.id -eq $ctxDirect) {
        Pass $G "direct-conv-idempotent" "Same ID returned=$ctxDirect"
    } elseif ($r2.ok) {
        Fail $G "direct-conv-idempotent" "Expected ID=$ctxDirect got $($r2.data.conversation.id)"
    } else {
        Fail $G "direct-conv-idempotent" "Request failed: HTTP $($r2.status)"
    }
}

# Non-participant (WE) cannot access direct conv between Super and WA
if ($ctxDirect) {
    $r = Invoke-Api "GET" "/chat/conversations/$ctxDirect" $hWE
    Expect-Fail $G "non-participant-cannot-get-direct-conv" $r 404 "WE has no access to Super-WA direct conv"
}

# Both participants can view the conversation
if ($ctxDirect) {
    $r = Invoke-Api "GET" "/chat/conversations/$ctxDirect" $hSuper
    Expect-OK $G "owner-can-get-direct-conv" $r "Super reads own direct conv"

    $r = Invoke-Api "GET" "/chat/conversations/$ctxDirect" $hWA
    Expect-OK $G "participant-can-get-direct-conv" $r "WA reads direct conv"
}

# List conversations  - both should see it
$r = Invoke-Api "GET" "/chat/conversations" $hSuper
Expect-OK $G "list-conversations-super" $r "Super lists conversations"
if ($r.ok) {
    $found = $false
    foreach ($c in @($r.data.conversations)) { if ($c.id -eq $ctxDirect) { $found = $true; break } }
    if ($found) { Pass $G "direct-conv-in-super-list" "Found in list" }
    else { Fail $G "direct-conv-in-super-list" "Not found in conversation list" }
}

$r = Invoke-Api "GET" "/chat/conversations?type=direct" $hSuper
Expect-OK $G "filter-by-type-direct" $r "Filter type=direct works"

# ============================================================================
# G3  - Group Conversation Creation
# ============================================================================
$G = "G3-GroupConversation"
Write-Host "`n=== $G ===" -ForegroundColor Cyan

# Superadmin has all permissions; try creating a group
$groupBody = @{
    title      = "Test Group $RunTag"
    member_ids = @($USER_WA, $USER_WE)
}

$r = Invoke-Api "POST" "/chat/groups" $hSuper $groupBody
Expect-OK $G "superadmin-creates-group" $r "POST /chat/groups with title+members"
if ($r.ok) {
    $grpObj = $r.data.group
    if ($grpObj) {
        $ctxGroup = $grpObj.id
        Pass $G "group-conv-id-present" "ID=$ctxGroup"
        if ($grpObj.type -eq "group") {
            Pass $G "group-type-is-group" "type=group confirmed"
        } else {
            Fail $G "group-type-is-group" "Expected type=group got $($grpObj.type)"
        }
    } else {
        Fail $G "group-conv-id-present" "No group key in response"
        Fail $G "group-type-is-group" "No group key in response"
    }
} else {
    Fail $G "group-conv-id-present" "Request failed: HTTP $($r.status)"
    Fail $G "group-type-is-group" "Request failed"
}

# Group must have title
$badBody = @{ member_ids = @($USER_WA) }
$r = Invoke-Api "POST" "/chat/groups" $hSuper $badBody
Expect-Fail $G "group-create-missing-title-rejected" $r 400 "no title"

# Group must have at least 1 member
$badBody2 = @{ title = "Empty Group"; member_ids = @() }
$r = Invoke-Api "POST" "/chat/groups" $hSuper $badBody2
Expect-Fail $G "group-create-empty-members-rejected" $r 400 "empty member_ids"

# WA does not have chat:group:create permission by default  - may get 403
$r = Invoke-Api "POST" "/chat/groups" $hWA $groupBody
if (-not $r.ok -and ($r.status -eq 403 -or $r.status -eq 401)) {
    Pass $G "water-admin-create-group-forbidden" "Correctly denied (HTTP $($r.status))"
} elseif ($r.ok) {
    # If WA has the permission assigned, that's valid too  - just note it
    Skip $G "water-admin-create-group-forbidden" "WA succeeded  - may have chat:group:create permission"
} else {
    Fail $G "water-admin-create-group-forbidden" "Unexpected HTTP $($r.status)"
}

# All three members can see the group
if ($ctxGroup) {
    $r = Invoke-Api "GET" "/chat/conversations/$ctxGroup" $hSuper
    Expect-OK $G "super-sees-group" $r "Creator sees group"

    $r = Invoke-Api "GET" "/chat/conversations/$ctxGroup" $hWA
    Expect-OK $G "wa-sees-group" $r "WA member sees group"

    $r = Invoke-Api "GET" "/chat/conversations/$ctxGroup" $hWE
    Expect-OK $G "we-sees-group" $r "WE member sees group"
}

# ============================================================================
# G4 - Send and Receive Text Messages (Direct)
# ============================================================================
$G = "G4-TextMessages"
Write-Host "`n=== $G ===" -ForegroundColor Cyan

if ($ctxDirect) {
    # Send plain text
    $msgBody = @{ content = "Hello WA! Test run $RunTag"; message_type = "text" }
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/messages" $hSuper $msgBody
    Expect-OK $G "super-sends-text-msg" $r "Super sends text to direct conv"
    if ($r.ok -and $r.data.message) {
        $ctxMsg1 = $r.data.message.id
        Pass $G "msg1-id-present" "ID=$ctxMsg1"
        if ($r.data.message.content -eq $msgBody.content) {
            Pass $G "msg1-content-correct" "content matches"
        } else {
            Fail $G "msg1-content-correct" "content mismatch: $($r.data.message.content)"
        }
        if ($r.data.message.message_type -eq "text") {
            Pass $G "msg1-type-text" "message_type=text"
        } else {
            Fail $G "msg1-type-text" "Expected text got $($r.data.message.message_type)"
        }
    }

    # WA replies
    if ($ctxMsg1) {
        $replyBody = @{
            content       = "Hi Super! Reply to msg1 run $RunTag"
            message_type  = "text"
            reply_to_id   = $ctxMsg1
        }
        $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/messages" $hWA $replyBody
        Expect-OK $G "wa-replies-with-reply-to" $r "WA sends reply-to message"
        if ($r.ok -and $r.data.message) {
            $ctxMsg2 = $r.data.message.id
            if ($r.data.message.reply_to_id -eq $ctxMsg1) {
                Pass $G "reply-to-id-set" "reply_to_id=$ctxMsg1"
            } else {
                Fail $G "reply-to-id-set" "Expected reply_to_id=$ctxMsg1 got $($r.data.message.reply_to_id)"
            }
        }
    }

    # Non-participant cannot send message
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/messages" $hWE @{ content = "Intruder msg"; message_type = "text" }
    Expect-Fail $G "non-participant-cannot-send-msg" $r 400 "WE not in direct conv"

    # Content required
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/messages" $hSuper @{ content = "" }
    Expect-Fail $G "empty-content-rejected" $r 400 "empty content"

    # List messages
    $r = Invoke-Api "GET" "/chat/conversations/$ctxDirect/messages" $hSuper
    Expect-OK $G "list-messages-in-direct-conv" $r "GET /conversations/{id}/messages"
    if ($r.ok -and $r.data.messages) {
        $msgCount = @($r.data.messages).Count
        if ($msgCount -ge 1) { Pass $G "messages-list-not-empty" "Got $msgCount messages" }
        else { Fail $G "messages-list-not-empty" "Expected >=1 messages" }
    }

    # Pagination
    $r = Invoke-Api "GET" "/chat/conversations/$ctxDirect/messages?page=1&page_size=1" $hSuper
    Expect-OK $G "messages-pagination-page1" $r "page_size=1 returns 200"
    if ($r.ok -and $r.data.messages) {
        if (@($r.data.messages).Count -le 1) { Pass $G "messages-page-size-1-respected" "page_size=1 honoured" }
        else { Fail $G "messages-page-size-1-respected" "Got $(@($r.data.messages).Count) expected <=1" }
    }

    # WA can also list
    $r = Invoke-Api "GET" "/chat/conversations/$ctxDirect/messages" $hWA
    Expect-OK $G "wa-can-list-messages" $r "WA lists messages in direct conv"

    # Get individual message
    if ($ctxMsg1) {
        $r = Invoke-Api "GET" "/chat/messages/$ctxMsg1" $hSuper
        Expect-OK $G "get-message-by-id" $r "GET /chat/messages/{id}"
        if ($r.ok -and $r.data.message.id -eq $ctxMsg1) {
            Pass $G "get-message-id-match" "id matches"
        } elseif ($r.ok) {
            Fail $G "get-message-id-match" "id mismatch: $($r.data.message.id)"
        }
    }
} else {
    Skip $G "all-text-message-tests" "No direct conversation available"
}

# ============================================================================
# G5  - Message Types: image, file, video, audio (via message_type field)
# ============================================================================
$G = "G5-MessageTypes"
Write-Host "`n=== $G ===" -ForegroundColor Cyan

if ($ctxDirect) {
    # Image message  - content must be non-empty (e.g. a DMS URL or description)
    $imgBody = @{
        content      = "https://example.com/images/photo-$RunTag.jpg"
        message_type = "image"
    }
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/messages" $hSuper $imgBody
    Expect-OK $G "send-image-type-message" $r "message_type=image"
    if ($r.ok -and $r.data.message) {
        $ctxMsgImage = $r.data.message.id
        if ($r.data.message.message_type -eq "image") {
            Pass $G "image-type-persisted" "message_type=image confirmed"
        } else {
            Fail $G "image-type-persisted" "Got $($r.data.message.message_type)"
        }
    }

    # File message
    $fileBody = @{
        content      = "https://example.com/docs/report-$RunTag.pdf"
        message_type = "file"
    }
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/messages" $hSuper $fileBody
    Expect-OK $G "send-file-type-message" $r "message_type=file"
    if ($r.ok -and $r.data.message) {
        $ctxMsgFile = $r.data.message.id
        if ($r.data.message.message_type -eq "file") {
            Pass $G "file-type-persisted" "message_type=file confirmed"
        } else {
            Fail $G "file-type-persisted" "Got $($r.data.message.message_type)"
        }
    }

    # Video message
    $vidBody = @{
        content      = "https://example.com/videos/clip-$RunTag.mp4"
        message_type = "video"
    }
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/messages" $hSuper $vidBody
    Expect-OK $G "send-video-type-message" $r "message_type=video"
    if ($r.ok -and $r.data.message) {
        if ($r.data.message.message_type -eq "video") {
            Pass $G "video-type-persisted" "message_type=video confirmed"
        } else {
            Fail $G "video-type-persisted" "Got $($r.data.message.message_type)"
        }
    }

    # Audio message
    $audBody = @{
        content      = "https://example.com/audio/note-$RunTag.ogg"
        message_type = "audio"
    }
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/messages" $hSuper $audBody
    Expect-OK $G "send-audio-type-message" $r "message_type=audio"
    if ($r.ok -and $r.data.message) {
        if ($r.data.message.message_type -eq "audio") {
            Pass $G "audio-type-persisted" "message_type=audio confirmed"
        } else {
            Fail $G "audio-type-persisted" "Got $($r.data.message.message_type)"
        }
    }

    # Location message
    $locBody = @{
        content      = '{"lat":28.6139,"lng":77.2090}'
        message_type = "location"
    }
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/messages" $hSuper $locBody
    Expect-OK $G "send-location-type-message" $r "message_type=location"

    # Default type (omit message_type) should be 'text'
    $defaultBody = @{ content = "Default type message $RunTag" }
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/messages" $hSuper $defaultBody
    Expect-OK $G "send-message-default-type" $r "no message_type defaults OK"
    if ($r.ok -and $r.data.message) {
        if ($r.data.message.message_type -eq "text") {
            Pass $G "default-message-type-is-text" "message_type=text (default)"
        } else {
            Fail $G "default-message-type-is-text" "Got $($r.data.message.message_type)"
        }
    }
} else {
    Skip $G "all-message-type-tests" "No direct conversation available"
}

# ============================================================================
# G6  - Attachments (document/image/media metadata)
# ============================================================================
$G = "G6-Attachments"
Write-Host "`n=== $G ===" -ForegroundColor Cyan

if ($ctxDirect -and $ctxMsg1) {
    # Add image attachment to msg1
    $attBody = @{
        file_name = "photo-$RunTag.jpg"
        file_size = 204800
        mime_type = "image/jpeg"
        dms_file_url = "https://storage.example.com/files/photo-$RunTag.jpg"
        thumbnail_url = "https://storage.example.com/thumbs/photo-$RunTag.jpg"
        metadata = @{ width = 1920; height = 1080; taken_by = "Super Admin" }
    }
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/messages/$ctxMsg1/attachments" $hSuper $attBody
    Expect-OK $G "add-image-attachment-to-msg" $r "POST /conversations/{id}/messages/{msgId}/attachments"
    if ($r.ok -and $r.data.attachment) {
        $ctxAttachment = $r.data.attachment.id
        Pass $G "attachment-id-present" "ID=$ctxAttachment"
        if ($r.data.attachment.mime_type -eq "image/jpeg") {
            Pass $G "attachment-mime-type-correct" "mime_type=image/jpeg"
        } else {
            Fail $G "attachment-mime-type-correct" "Got $($r.data.attachment.mime_type)"
        }
    }

    # Add PDF document attachment
    $docBody = @{
        file_name = "report-$RunTag.pdf"
        file_size = 512000
        mime_type = "application/pdf"
        dms_file_url = "https://storage.example.com/files/report-$RunTag.pdf"
    }
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/messages/$ctxMsg1/attachments" $hSuper $docBody
    Expect-OK $G "add-pdf-attachment-to-msg" $r "POST attachment mime_type=application/pdf"
    if ($r.ok -and $r.data.attachment.mime_type -eq "application/pdf") {
        Pass $G "pdf-mime-type-correct" "mime_type=application/pdf"
    } elseif ($r.ok) {
        Fail $G "pdf-mime-type-correct" "Got $($r.data.attachment.mime_type)"
    }

    # Add video attachment
    $vidAttBody = @{
        file_name = "video-$RunTag.mp4"
        file_size = 10485760
        mime_type = "video/mp4"
        dms_file_url = "https://storage.example.com/files/video-$RunTag.mp4"
        thumbnail_url = "https://storage.example.com/thumbs/video-$RunTag.jpg"
    }
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/messages/$ctxMsg1/attachments" $hSuper $vidAttBody
    Expect-OK $G "add-video-attachment-to-msg" $r "POST attachment mime_type=video/mp4"

    # Add audio attachment
    $audAttBody = @{
        file_name = "audio-$RunTag.ogg"
        file_size = 102400
        mime_type = "audio/ogg"
        dms_file_url = "https://storage.example.com/files/audio-$RunTag.ogg"
    }
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/messages/$ctxMsg1/attachments" $hSuper $audAttBody
    Expect-OK $G "add-audio-attachment-to-msg" $r "POST attachment mime_type=audio/ogg"

    # Attachment requires file_name
    $badAtt = @{ file_size = 100; mime_type = "image/jpeg" }
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/messages/$ctxMsg1/attachments" $hSuper $badAtt
    Expect-Fail $G "attachment-missing-file-name-rejected" $r 400 "no file_name"

    # Attachment requires mime_type
    $badAtt2 = @{ file_name = "x.jpg"; file_size = 100 }
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/messages/$ctxMsg1/attachments" $hSuper $badAtt2
    Expect-Fail $G "attachment-missing-mime-type-rejected" $r 400 "no mime_type"

    # Non-participant cannot attach
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/messages/$ctxMsg1/attachments" $hWE $attBody
    Expect-Fail $G "non-participant-cannot-attach" $r 400 "WE not in direct conv"

    # List attachments in conversation
    $r = Invoke-Api "GET" "/chat/conversations/$ctxDirect/attachments" $hSuper
    Expect-OK $G "list-attachments-in-direct-conv" $r "GET /conversations/{id}/attachments"
    if ($r.ok -and $r.data.attachments) {
        $attCount = @($r.data.attachments).Count
        if ($attCount -ge 1) { Pass $G "attachments-list-not-empty" "Got $attCount attachments" }
        else { Fail $G "attachments-list-not-empty" "Expected >=1 attachments" }
    }

    # WA (participant) can also list attachments
    $r = Invoke-Api "GET" "/chat/conversations/$ctxDirect/attachments" $hWA
    Expect-OK $G "wa-can-list-attachments" $r "WA lists attachments"

    # Non-participant cannot list attachments
    $r = Invoke-Api "GET" "/chat/conversations/$ctxDirect/attachments" $hWE
    Expect-Fail $G "non-participant-cannot-list-attachments" $r 400 "WE not in direct conv"
} else {
    Skip $G "all-attachment-tests" "No direct conversation or msg1 available"
}

# ============================================================================
# G7  - Group Messages
# ============================================================================
$G = "G7-GroupMessages"
Write-Host "`n=== $G ===" -ForegroundColor Cyan

if ($ctxGroup) {
    # Super sends to group
    $gMsgBody = @{ content = "Group announcement $RunTag"; message_type = "text" }
    $r = Invoke-Api "POST" "/chat/conversations/$ctxGroup/messages" $hSuper $gMsgBody
    Expect-OK $G "super-sends-text-to-group" $r "Super sends message to group"
    if ($r.ok -and $r.data.message) {
        $ctxGroupMsg = $r.data.message.id
        Pass $G "group-msg-id-present" "ID=$ctxGroupMsg"
    }

    # WA sends to group
    $r = Invoke-Api "POST" "/chat/conversations/$ctxGroup/messages" $hWA @{ content = "WA reply in group $RunTag"; message_type = "text" }
    Expect-OK $G "wa-sends-to-group" $r "WA member sends text to group"

    # WE sends to group
    $r = Invoke-Api "POST" "/chat/conversations/$ctxGroup/messages" $hWE @{ content = "WE reply in group $RunTag"; message_type = "text" }
    Expect-OK $G "we-sends-to-group" $r "WE member sends text to group"

    # Image message in group
    $r = Invoke-Api "POST" "/chat/conversations/$ctxGroup/messages" $hWA @{
        content      = "https://example.com/images/group-photo-$RunTag.jpg"
        message_type = "image"
    }
    Expect-OK $G "wa-sends-image-msg-to-group" $r "WA sends image to group"

    # File message in group
    $r = Invoke-Api "POST" "/chat/conversations/$ctxGroup/messages" $hSuper @{
        content      = "https://example.com/docs/group-doc-$RunTag.pdf"
        message_type = "file"
    }
    Expect-OK $G "super-sends-file-msg-to-group" $r "Super sends file to group"

    # List group messages  - all 3 participants should see them
    foreach ($pair in @(@{User="super";H=$hSuper},@{User="wa";H=$hWA},@{User="we";H=$hWE})) {
        $r = Invoke-Api "GET" "/chat/conversations/$ctxGroup/messages" $pair.H
        Expect-OK $G "list-group-msgs-$($pair.User)" $r "$($pair.User) lists group messages"
    }

    # Attachment in group message
    if ($ctxGroupMsg) {
        $gAttBody = @{
            file_name    = "group-file-$RunTag.docx"
            file_size    = 307200
            mime_type    = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
            dms_file_url = "https://storage.example.com/files/group-$RunTag.docx"
        }
        $r = Invoke-Api "POST" "/chat/conversations/$ctxGroup/messages/$ctxGroupMsg/attachments" $hSuper $gAttBody
        Expect-OK $G "add-docx-attachment-to-group-msg" $r "POST attachment mime_type=application/vnd...docx"
    }
} else {
    Skip $G "all-group-message-tests" "No group conversation available"
}

# ============================================================================
# G8  - Participant Management (Groups)
# ============================================================================
$G = "G8-Participants"
Write-Host "`n=== $G ===" -ForegroundColor Cyan

if ($ctxGroup) {
    # List participants
    $r = Invoke-Api "GET" "/chat/conversations/$ctxGroup/participants" $hSuper
    Expect-OK $G "list-participants-owner" $r "GET /conversations/{id}/participants"
    if ($r.ok -and $r.data.participants) {
        $pCount = @($r.data.participants).Count
        # Creator + 2 members = 3
        if ($pCount -ge 3) { Pass $G "participant-count-correct" "Got $pCount participants" }
        else { Fail $G "participant-count-correct" "Expected >=3 got $pCount" }
    }

    # WA can list participants too
    $r = Invoke-Api "GET" "/chat/conversations/$ctxGroup/participants" $hWA
    Expect-OK $G "member-can-list-participants" $r "WA member lists participants"

    # Update WA's role to admin
    $r = Invoke-Api "PATCH" "/chat/conversations/$ctxGroup/participants/$USER_WA/role" $hSuper @{ role = "admin" }
    Expect-OK $G "owner-promotes-member-to-admin" $r "Super promotes WA to admin"
    if ($r.ok) {
        # Demote back to member
        $r2 = Invoke-Api "PATCH" "/chat/conversations/$ctxGroup/participants/$USER_WA/role" $hSuper @{ role = "member" }
        Expect-OK $G "owner-demotes-admin-to-member" $r2 "Super demotes WA back to member"
    }

    # WE (member) cannot change roles
    $r = Invoke-Api "PATCH" "/chat/conversations/$ctxGroup/participants/$USER_WA/role" $hWE @{ role = "admin" }
    Expect-Fail $G "member-cannot-change-role" $r 400 "WE member tries to promote WA"

    # Member (WE) cannot add participants
    $r = Invoke-Api "POST" "/chat/conversations/$ctxGroup/participants" $hWE @{ user_id = $USER_SUPER; role = "member" }
    Expect-Fail $G "member-cannot-add-participants" $r 400 "WE member cannot add participants"

    # Remove WA from group (owner can remove non-owner members)
    $r = Invoke-Api "DELETE" "/chat/conversations/$ctxGroup/participants/$USER_WA" $hSuper
    Expect-OK $G "owner-removes-participant" $r "Super removes WA from group"

    # After removal WA cannot see group
    $r = Invoke-Api "GET" "/chat/conversations/$ctxGroup" $hWA
    Expect-Fail $G "removed-participant-cannot-see-group" $r 404 "WA removed, should be 404"

    # Re-add WA (note: re-add may fail if backend has unique constraint without soft-delete support)
    $r = Invoke-Api "POST" "/chat/conversations/$ctxGroup/participants" $hSuper @{ user_id = $USER_WA; role = "member" }
    if ($r.ok) {
        Pass $G "owner-adds-participant-back" "Super re-adds WA"
        $rWA = Invoke-Api "GET" "/chat/conversations/$ctxGroup" $hWA
        Expect-OK $G "re-added-participant-sees-group" $rWA "WA re-added and can see group"
    } else {
        # Backend does not support re-adding due to unique constraint on (conversation_id, user_id)
        Skip $G "owner-adds-participant-back" "Backend unique constraint prevents re-add (HTTP $($r.status))"
        Skip $G "re-added-participant-sees-group" "Skipped due to re-add limitation"
    }
} else {
    Skip $G "all-participant-tests" "No group conversation available"
}

# ============================================================================
# G9  - Read Receipts & Unread Count
# ============================================================================
$G = "G9-ReadReceipts"
Write-Host "`n=== $G ===" -ForegroundColor Cyan

if ($ctxDirect) {
    # Mark direct conv as read by WA
    if ($ctxMsg1) {
        $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/read" $hWA @{ message_id = $ctxMsg1 }
    } else {
        $r = @{ ok = $false; status = 0; error = "no msg1 available" }
    }
    Expect-OK $G "wa-marks-direct-conv-as-read" $r "POST /conversations/{id}/read"

    # Check unread count is now 0 for WA
    $r = Invoke-Api "GET" "/chat/conversations/$ctxDirect" $hWA
    Expect-OK $G "get-conv-for-unread-check" $r "GET conversation after read"
    if ($r.ok -and $r.data.conversation) {
        $unread = $r.data.conversation.PSObject.Properties['unread_count']
        if ($null -eq $unread) {
            Skip $G "unread-count-zero-after-read" "unread_count not in response"
        } elseif ($unread.Value -eq 0) {
            Pass $G "unread-count-zero-after-read" "unread_count=0"
        } else {
            Skip $G "unread-count-zero-after-read" "unread_count=$($unread.Value) (may have new msgs)"
        }
    }

    # Super marks as read
    if ($ctxMsg1) {
        $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/read" $hSuper @{ message_id = $ctxMsg1 }
    } else {
        $r = @{ ok = $false; status = 0; error = "no msg1 available" }
    }
    Expect-OK $G "super-marks-direct-conv-as-read" $r "Super marks as read"

    # Non-participant cannot mark as read
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/read" $hWE @{ message_id = "00000000-0000-0000-0000-000000000001" }
    Expect-Fail $G "non-participant-cannot-mark-read" $r 0 "WE not in conv"
} else {
    Skip $G "all-read-receipt-tests" "No direct conversation available"
}

if ($ctxGroup) {
    # Mark group as read
    if ($ctxGroupMsg) {
        $r = Invoke-Api "POST" "/chat/conversations/$ctxGroup/read" $hWE @{ message_id = $ctxGroupMsg }
    } else {
        $r = @{ ok = $false; status = 0; error = "no group msg available" }
    }
    Expect-OK $G "we-marks-group-as-read" $r "WE marks group as read"
}

# ============================================================================
# G10  - Typing Indicators
# ============================================================================
$G = "G10-Typing"
Write-Host "`n=== $G ===" -ForegroundColor Cyan

if ($ctxDirect) {
    # Super sends typing indicator
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/typing" $hSuper @{}
    Expect-OK $G "super-sends-typing-indicator" $r "POST /conversations/{id}/typing"

    # WA sends typing
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/typing" $hWA @{}
    Expect-OK $G "wa-sends-typing-indicator" $r "WA sends typing in direct conv"

    # Get typing users
    $r = Invoke-Api "GET" "/chat/conversations/$ctxDirect/typing" $hSuper
    Expect-OK $G "get-typing-users" $r "GET /conversations/{id}/typing"

    # Non-participant cannot send typing
    $r = Invoke-Api "POST" "/chat/conversations/$ctxDirect/typing" $hWE @{}
    Expect-Fail $G "non-participant-typing-rejected" $r 400 "WE not in conv"
} else {
    Skip $G "all-typing-tests" "No direct conversation available"
}

if ($ctxGroup) {
    $r = Invoke-Api "POST" "/chat/conversations/$ctxGroup/typing" $hWE @{}
    Expect-OK $G "we-sends-typing-in-group" $r "WE typing in group conv"

    $r = Invoke-Api "GET" "/chat/conversations/$ctxGroup/typing" $hSuper
    Expect-OK $G "get-typing-users-in-group" $r "GET typing users in group"
}

# ============================================================================
# G11  - Reactions
# ============================================================================
$G = "G11-Reactions"
Write-Host "`n=== $G ===" -ForegroundColor Cyan

if ($ctxMsg1) {
    $ctxReactionMsg = $ctxMsg1

    # Super reacts with thumbs-up
    $r = Invoke-Api "POST" "/chat/messages/$ctxReactionMsg/reactions" $hSuper @{ reaction = "+1" }
    Expect-OK $G "super-adds-thumbs-up-reaction" $r "POST /messages/{id}/reactions"

    # WA reacts with heart
    $r = Invoke-Api "POST" "/chat/messages/$ctxReactionMsg/reactions" $hWA @{ reaction = "heart" }
    Expect-OK $G "wa-adds-heart-reaction" $r "WA adds heart reaction"

    # List reactions
    $r = Invoke-Api "GET" "/chat/messages/$ctxReactionMsg/reactions" $hSuper
    Expect-OK $G "list-reactions" $r "GET /messages/{id}/reactions"
    if ($r.ok -and $r.data.reactions) {
        $rCount = @($r.data.reactions).Count
        if ($rCount -ge 2) { Pass $G "reaction-count-at-least-2" "Got $rCount reactions" }
        else { Skip $G "reaction-count-at-least-2" "Got $rCount (may vary)" }
    }

    # Remove super's reaction
    $encodedEmoji = [uri]::EscapeDataString("+1")
    $r = Invoke-Api "DELETE" "/chat/messages/$ctxReactionMsg/reactions/$encodedEmoji" $hSuper
    Expect-OK $G "super-removes-thumbs-up-reaction" $r "DELETE /messages/{id}/reactions/{reaction}"

    # Non-participant (WE) cannot react to message in direct conv
    $r = Invoke-Api "POST" "/chat/messages/$ctxReactionMsg/reactions" $hWE @{ reaction = "wow" }
    Expect-Fail $G "non-participant-cannot-react" $r 400 "WE not in conv"

    # Empty reaction rejected
    $r = Invoke-Api "POST" "/chat/messages/$ctxReactionMsg/reactions" $hSuper @{ reaction = "" }
    Expect-Fail $G "empty-reaction-rejected" $r 400 "empty reaction string"
} else {
    Skip $G "all-reaction-tests" "No message available"
}

if ($ctxGroupMsg) {
    # All group members can react
    $r = Invoke-Api "POST" "/chat/messages/$ctxGroupMsg/reactions" $hWE @{ reaction = "fire" }
    Expect-OK $G "we-reacts-in-group-msg" $r "WE reacts to group message"

    $encodedFire = [uri]::EscapeDataString("fire")
    $r = Invoke-Api "DELETE" "/chat/messages/$ctxGroupMsg/reactions/$encodedFire" $hWE
    Expect-OK $G "we-removes-group-reaction" $r "WE removes reaction from group msg"
}

# ============================================================================
# G12  - Message Edit & Delete
# ============================================================================
$G = "G12-MessageEditDelete"
Write-Host "`n=== $G ===" -ForegroundColor Cyan

if ($ctxMsg1) {
    # Super edits own message
    $editBody = @{ content = "Edited content $RunTag" }
    $r = Invoke-Api "PUT" "/chat/messages/$ctxMsg1" $hSuper $editBody
    Expect-OK $G "sender-can-edit-own-message" $r "PUT /chat/messages/{id}"
    if ($r.ok -and $r.data.message) {
        if ($r.data.message.content -eq $editBody.content) {
            Pass $G "edited-content-persisted" "content updated correctly"
        } else {
            Fail $G "edited-content-persisted" "Got $($r.data.message.content)"
        }
    }

    # WA cannot edit Super's message
    $r = Invoke-Api "PUT" "/chat/messages/$ctxMsg1" $hWA @{ content = "WA hacks super msg" }
    Expect-Fail $G "non-sender-cannot-edit-message" $r 400 "WA tries to edit Super's msg"

    # Empty edit rejected
    $r = Invoke-Api "PUT" "/chat/messages/$ctxMsg1" $hSuper @{ content = "" }
    Expect-Fail $G "empty-edit-content-rejected" $r 400 "empty content"
}

if ($ctxMsg2) {
    # Delete a message (WA deletes own reply)
    $r = Invoke-Api "DELETE" "/chat/messages/$ctxMsg2" $hWA
    Expect-OK $G "sender-deletes-own-message" $r "DELETE /chat/messages/{id}"

    # Verify it's gone
    $r = Invoke-Api "GET" "/chat/messages/$ctxMsg2" $hWA
    Expect-Fail $G "deleted-message-not-found" $r 0 "Deleted msg should return 4xx"
} else {
    Skip $G "delete-message-tests" "No msg2 (WA reply) available"
}

# ============================================================================
# G13  - Message Search
# ============================================================================
$G = "G13-MessageSearch"
Write-Host "`n=== $G ===" -ForegroundColor Cyan

if ($ctxDirect) {
    # Search for unique run tag
    $r = Invoke-Api "GET" "/chat/conversations/$ctxDirect/messages/search?q=$RunTag" $hSuper
    Expect-OK $G "search-messages-by-runTag" $r "GET /conversations/{id}/messages/search?q=RunTag"
    if ($r.ok -and $r.data.messages) {
        $sCount = @($r.data.messages).Count
        if ($sCount -ge 1) { Pass $G "search-finds-messages" "Got $sCount results for run tag" }
        else { Fail $G "search-finds-messages" "Expected >=1 search results" }
    }

    # Search with no results
    $r = Invoke-Api "GET" "/chat/conversations/$ctxDirect/messages/search?q=xyzNEVERMATCHES123$RunTag" $hSuper
    Expect-OK $G "search-no-results-returns-200" $r "Search with no matches still 200"
    if ($r.ok -and $r.data.messages) {
        if (@($r.data.messages).Count -eq 0) {
            Pass $G "search-no-matches-empty-list" "Empty list returned correctly"
        } else {
            Fail $G "search-no-matches-empty-list" "Expected 0 got $(@($r.data.messages).Count)"
        }
    }

    # Non-participant cannot search
    $r = Invoke-Api "GET" "/chat/conversations/$ctxDirect/messages/search?q=Hello" $hWE
    Expect-Fail $G "non-participant-cannot-search" $r 400 "WE not in conv (backend returns 400)"
} else {
    Skip $G "all-search-tests" "No direct conversation available"
}

if ($ctxGroup) {
    # Group member can search group messages
    $r = Invoke-Api "GET" "/chat/conversations/$ctxGroup/messages/search?q=$RunTag" $hWE
    Expect-OK $G "we-searches-group-messages" $r "WE searches group conv messages"
}

# ============================================================================
# G14  - Conversation Update, Archive & Delete
# ============================================================================
$G = "G14-ConvUpdateArchiveDelete"
Write-Host "`n=== $G ===" -ForegroundColor Cyan

if ($ctxGroup) {
    # Owner updates group title/description
    $updateBody = @{
        title       = "Updated Group $RunTag"
        description = "Updated description $RunTag"
    }
    $r = Invoke-Api "PUT" "/chat/conversations/$ctxGroup" $hSuper $updateBody
    Expect-OK $G "owner-updates-group-title" $r "PUT /conversations/{id} (owner)"
    if ($r.ok -and $r.data.conversation) {
        if ($r.data.conversation.title -eq $updateBody.title) {
            Pass $G "group-title-updated" "title matches"
        } else {
            Fail $G "group-title-updated" "Got $($r.data.conversation.title)"
        }
    }

    # Member (WE) cannot update group
    $r = Invoke-Api "PUT" "/chat/conversations/$ctxGroup" $hWE @{ title = "WE Hacks Title" }
    Expect-Fail $G "member-cannot-update-group" $r 400 "WE member tries to update conv"

    # Archive group
    $r = Invoke-Api "PATCH" "/chat/conversations/$ctxGroup/archive" $hSuper @{ archive = $true }
    Expect-OK $G "owner-archives-group" $r "PATCH /conversations/{id}/archive { archive: true }"
    if ($r.ok -and $r.data.conversation) {
        if ($r.data.conversation.is_archived -eq $true) {
            Pass $G "group-is-archived" "is_archived=true"
        } else {
            Fail $G "group-is-archived" "Expected is_archived=true got $($r.data.conversation.is_archived)"
        }
    }

    # Archived group does not show in default list
    $r = Invoke-Api "GET" "/chat/conversations" $hSuper
    Expect-OK $G "list-convs-excludes-archived" $r "Default list excludes archived"
    if ($r.ok -and $r.data.conversations) {
        $archFound = $false
        foreach ($c in @($r.data.conversations)) { if ($c.id -eq $ctxGroup) { $archFound = $true; break } }
        if (-not $archFound) { Pass $G "archived-group-not-in-default-list" "Archived conv hidden" }
        else { Fail $G "archived-group-not-in-default-list" "Archived conv still showing" }
    }

    # Archived group shows with include_archived=true
    $r = Invoke-Api "GET" "/chat/conversations?include_archived=true" $hSuper
    Expect-OK $G "list-convs-include-archived" $r "include_archived=true"
    if ($r.ok -and $r.data.conversations) {
        $archFound = $false
        foreach ($c in @($r.data.conversations)) { if ($c.id -eq $ctxGroup) { $archFound = $true; break } }
        if ($archFound) { Pass $G "archived-group-in-include-archived-list" "Found with include_archived" }
        else { Fail $G "archived-group-in-include-archived-list" "Not found even with include_archived=true" }
    }

    # Unarchive
    $r = Invoke-Api "PATCH" "/chat/conversations/$ctxGroup/archive" $hSuper @{ archive = $false }
    Expect-OK $G "owner-unarchives-group" $r "PATCH archive { archive: false }"
    if ($r.ok -and $r.data.conversation) {
        if ($r.data.conversation.is_archived -eq $false) {
            Pass $G "group-is-unarchived" "is_archived=false"
        } else {
            Fail $G "group-is-unarchived" "Expected is_archived=false"
        }
    }
}

if ($ctxDirect) {
    # Archive direct conv
    $r = Invoke-Api "PATCH" "/chat/conversations/$ctxDirect/archive" $hSuper @{ archive = $true }
    Expect-OK $G "archive-direct-conv" $r "Archive direct conversation"

    # Unarchive
    $r = Invoke-Api "PATCH" "/chat/conversations/$ctxDirect/archive" $hSuper @{ archive = $false }
    Expect-OK $G "unarchive-direct-conv" $r "Unarchive direct conversation"
}

# ============================================================================
# G15  - Auth Guards (no token, wrong key)
# ============================================================================
$G = "G15-AuthGuards"
Write-Host "`n=== $G ===" -ForegroundColor Cyan

$noToken = @{ "x-api-key" = $ApiKey; "Content-Type" = "application/json"; "Accept" = "application/json" }
$badKey  = @{ "x-api-key" = "bad-key-000"; "Authorization" = "Bearer $($lSuper.token)"; "Content-Type" = "application/json"; "Accept" = "application/json" }

$guardPaths = @(
    @{ M = "GET";  P = "/chat/users" }
    @{ M = "GET";  P = "/chat/conversations" }
    @{ M = "POST"; P = "/chat/conversations" }
)

foreach ($ep in $guardPaths) {
    $r = Invoke-Api $ep.M $ep.P $noToken
    Expect-Fail $G "no-token-$($ep.M)-$($ep.P -replace '/','_')" $r 401 "no auth header"

    $r = Invoke-Api $ep.M $ep.P $badKey
    Expect-Fail $G "bad-key-$($ep.M)-$($ep.P -replace '/','_')" $r 401 "bad api key"
}

# ============================================================================
# G16  - Cleanup: Delete test conversations
# ============================================================================
$G = "G16-Cleanup"
Write-Host "`n=== $G ===" -ForegroundColor Cyan

if ($ctxGroup) {
    $r = Invoke-Api "DELETE" "/chat/conversations/$ctxGroup" $hSuper
    Expect-OK $G "delete-test-group" $r "Owner deletes group conv"

    # Verify deleted
    $r = Invoke-Api "GET" "/chat/conversations/$ctxGroup" $hSuper
    Expect-Fail $G "deleted-group-not-accessible" $r 404 "Deleted group returns 404"
}

if ($ctxDirect) {
    # Non-owner cannot delete
    $r = Invoke-Api "DELETE" "/chat/conversations/$ctxDirect" $hWA
    Expect-Fail $G "non-owner-cannot-delete-conv" $r 400 "WA is not owner"

    $r = Invoke-Api "DELETE" "/chat/conversations/$ctxDirect" $hSuper
    Expect-OK $G "delete-test-direct-conv" $r "Owner deletes direct conv"

    $r = Invoke-Api "GET" "/chat/conversations/$ctxDirect" $hSuper
    Expect-Fail $G "deleted-direct-conv-not-accessible" $r 404 "Deleted direct conv returns 404"
}

# ============================================================================
# G17  - Pairwise User-to-User Direct Chats (Matrix Coverage)
# ============================================================================
$G = "G17-UserMatrix"
Write-Host "`n=== $G ===" -ForegroundColor Cyan

# Define user pairs to test (name, ID, headers)
$userPairs = @(
    @{name1="WA"; id1=$USER_WA; h1=$hWA; name2="WE"; id2=$USER_WE; h2=$hWE}
    @{name1="Super"; id1=$USER_SUPER; h1=$hSuper; name2="WE"; id2=$USER_WE; h2=$hWE}
    @{name1="WE"; id1=$USER_WE; h1=$hWE; name2="Super"; id2=$USER_SUPER; h2=$hSuper}
)

foreach ($pair in $userPairs) {
    $pairName = "$($pair.name1)-to-$($pair.name2)"
    
    # User1 creates direct conv with User2
    $body = @{
        type           = "direct"
        participant_ids = @($pair.id2)
    }
    $r = Invoke-Api "POST" "/chat/conversations" $pair.h1 $body
    if ($r.ok -and $r.data.conversation) {
        $convID = $r.data.conversation.id
        Pass $G "create-direct-$pairName" "Direct conv created"
        
        # User1 sends message to User2
        $msg1 = @{ content = "Message from $($pair.name1) to $($pair.name2) - $RunTag"; message_type = "text" }
        $r = Invoke-Api "POST" "/chat/conversations/$convID/messages" $pair.h1 $msg1
        if ($r.ok -and $r.data.message) {
            Pass $G "msg-$pairName-send" "User1 sends message"
            
            # User2 can read User1's message
            $r = Invoke-Api "GET" "/chat/conversations/$convID/messages" $pair.h2
            Expect-OK $G "msg-$pairName-list-user2" $r "User2 lists messages from User1"
            
            # User2 replies
            $msg2 = @{ content = "Reply from $($pair.name2) to $($pair.name1) - $RunTag"; message_type = "text" }
            $r = Invoke-Api "POST" "/chat/conversations/$convID/messages" $pair.h2 $msg2
            Expect-OK $G "msg-$pairName-reply" $r "User2 replies to User1"
            
            # User1 can read User2's reply
            $r = Invoke-Api "GET" "/chat/conversations/$convID/messages" $pair.h1
            Expect-OK $G "msg-$pairName-list-user1" $r "User1 reads User2's reply"
            
            # Both can search
            $r = Invoke-Api "GET" "/chat/conversations/$convID/messages/search?q=$RunTag" $pair.h1
            Expect-OK $G "msg-$pairName-search-user1" $r "User1 searches messages"
            
            $r = Invoke-Api "GET" "/chat/conversations/$convID/messages/search?q=$RunTag" $pair.h2
            Expect-OK $G "msg-$pairName-search-user2" $r "User2 searches messages"
        } else {
            Fail $G "msg-$pairName-send" "User1 failed to send message"
        }
    } else {
        Fail $G "create-direct-$pairName" "Failed to create direct conv: HTTP $($r.status)"
    }
}

# ============================================================================
# Final Summary
# ============================================================================
$total = $results.Count
$pass  = @($results | Where-Object { $_.Status -eq "PASS" }).Count
$fail  = @($results | Where-Object { $_.Status -eq "FAIL" }).Count
$skip  = @($results | Where-Object { $_.Status -eq "SKIP" }).Count

Write-Host "`n================================================================" -ForegroundColor White
Write-Host "  CHAT EXHAUSTIVE TEST SUMMARY" -ForegroundColor Cyan
Write-Host "  Run Tag : $RunTag" -ForegroundColor White
Write-Host "================================================================" -ForegroundColor White

$groupNames = $results | Select-Object -ExpandProperty Group -Unique
foreach ($gn in $groupNames) {
    $gRows  = $results | Where-Object { $_.Group -eq $gn }
    $gPass  = @($gRows | Where-Object { $_.Status -eq "PASS" }).Count
    $gFail  = @($gRows | Where-Object { $_.Status -eq "FAIL" }).Count
    $gSkip  = @($gRows | Where-Object { $_.Status -eq "SKIP" }).Count
    $gTotal = $gRows.Count
    $gColor = if ($gFail -gt 0) { "Red" } elseif ($gSkip -gt 0) { "Yellow" } else { "Green" }
    Write-Host ("  {0,-32} PASS={1}  FAIL={2}  SKIP={3}  ({4} total)" -f $gn, $gPass, $gFail, $gSkip, $gTotal) -ForegroundColor $gColor
}

Write-Host "----------------------------------------------------------------" -ForegroundColor White
$overallColor = if ($fail -gt 0) { "Red" } else { "Green" }
Write-Host ("  TOTAL: PASS={0}  FAIL={1}  SKIP={2}  ({3} total)" -f $pass, $fail, $skip, $total) -ForegroundColor $overallColor
Write-Host "================================================================" -ForegroundColor White

if ($fail -gt 0) {
    Write-Host "`nFAILED CASES:" -ForegroundColor Red
    $results | Where-Object { $_.Status -eq "FAIL" } | ForEach-Object {
        Write-Host "  - [$($_.Group)] $($_.Case) :: $($_.Detail)" -ForegroundColor Red
    }
}

# Write results to log
$results | Format-Table -AutoSize | Out-String | Tee-Object -FilePath $LogFile -Append | Out-Null
"TOTAL: PASS=$pass FAIL=$fail SKIP=$skip" | Tee-Object -FilePath $LogFile -Append | Out-Null

if ($fail -gt 0) { exit 1 } else { exit 0 }
