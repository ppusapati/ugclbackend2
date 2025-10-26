# File Upload Configuration Guide

## Overview

The backend supports two file storage methods:
- **Local File Storage**: For development environments
- **Google Cloud Storage (GCS)**: For production deployment

The system automatically detects the environment and uses the appropriate storage method.

---

## Development Environment (Local Storage)

### How it works
- Files are saved to `./uploads` directory on the server
- Files are served via `/uploads/` endpoint
- No Google Cloud credentials needed

### Setup
```bash
cd backend/v1
go run main.go
```

That's it! The system automatically uses local storage when:
- `USE_GCS` environment variable is NOT set to "true"
- `GOOGLE_APPLICATION_CREDENTIALS` is NOT set
- `K_SERVICE` is NOT set (not running on Cloud Run)

### File Access
Uploaded files are accessible at:
```
http://localhost:8080/uploads/[timestamp]-[filename]
```

---

## Production Environment (Google Cloud Storage)

### Prerequisites
1. **GCS Bucket**: Create a bucket in Google Cloud Console
   - Project ID: `ugcl-461407`
   - Current bucket: `sreeugcl`
   - Location: Choose based on your region

2. **Service Account**: Create a service account with permissions:
   - `Storage Object Admin` role
   - Download JSON key file

### Deployment Options

#### Option 1: Cloud Run (Recommended)
Cloud Run automatically provides credentials via metadata server.

**Deploy:**
```bash
# Set environment variable in Cloud Run
gcloud run deploy ugcl-backend \
  --source . \
  --set-env-vars USE_GCS=true \
  --allow-unauthenticated \
  --region us-central1
```

#### Option 2: App Engine
App Engine also provides automatic credentials.

**app.yaml:**
```yaml
runtime: go122
env_variables:
  USE_GCS: "true"
```

**Deploy:**
```bash
gcloud app deploy
```

#### Option 3: Compute Engine / VM
Requires explicit service account credentials.

**Setup:**
```bash
# Upload service account key to server
scp service-account-key.json user@server:/etc/secrets/

# Set environment variable
export GOOGLE_APPLICATION_CREDENTIALS="/etc/secrets/service-account-key.json"
export USE_GCS=true

# Run application
./ugcl-backend
```

#### Option 4: Docker / Kubernetes
Mount credentials as a secret.

**Dockerfile:**
```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o main .

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]
```

**Kubernetes Secret:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gcs-credentials
type: Opaque
data:
  key.json: <base64-encoded-service-account-key>
---
apiVersion: v1
kind: Pod
metadata:
  name: ugcl-backend
spec:
  containers:
  - name: backend
    image: gcr.io/ugcl-461407/backend:latest
    env:
    - name: GOOGLE_APPLICATION_CREDENTIALS
      value: /secrets/key.json
    - name: USE_GCS
      value: "true"
    volumeMounts:
    - name: gcs-key
      mountPath: /secrets
  volumes:
  - name: gcs-key
    secret:
      secretName: gcs-credentials
```

---

## Environment Variables

| Variable | Purpose | Values | Default |
|----------|---------|--------|---------|
| `USE_GCS` | Force GCS usage | `true` / `false` | Auto-detect |
| `GOOGLE_APPLICATION_CREDENTIALS` | Path to service account JSON | `/path/to/key.json` | - |
| `K_SERVICE` | Cloud Run indicator (auto-set) | Service name | - |

---

## Auto-Detection Logic

The system uses GCS when **any** of these conditions are true:

1. ✅ `USE_GCS=true` is explicitly set
2. ✅ `GOOGLE_APPLICATION_CREDENTIALS` is set
3. ✅ `K_SERVICE` is set (running on Cloud Run)

Otherwise, it falls back to local file storage.

**Implementation:** See `handlers/file_handler.go`

---

## File URLs

### Development (Local Storage)
```
http://localhost:8080/uploads/20251021-143022-photo.jpg
```

### Production (GCS)
```
https://storage.googleapis.com/sreeugcl/photo.jpg
```

---

## Testing

### Test Local Upload
```bash
curl -X POST http://localhost:8080/api/v1/files/upload \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -F "file=@test.jpg"

# Response:
{
  "url": "/uploads/20251021-143022-test.jpg",
  "filename": "20251021-143022-test.jpg"
}
```

### Test GCS Upload
```bash
curl -X POST https://your-app.run.app/api/v1/files/upload \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -F "file=@test.jpg"

# Response:
{
  "url": "https://storage.googleapis.com/sreeugcl/test.jpg"
}
```

---

## Troubleshooting

### Error: "failed to create GCS client"
**Cause**: Missing or invalid Google Cloud credentials

**Solutions:**
1. **Development**: System should auto-fallback to local storage
2. **Production**:
   - Verify `GOOGLE_APPLICATION_CREDENTIALS` points to valid JSON key
   - Check service account has `Storage Object Admin` role
   - Ensure bucket exists and is accessible

### Error: "failed to create upload directory"
**Cause**: Permission issues with local `./uploads` directory

**Solution:**
```bash
mkdir -p ./uploads
chmod 755 ./uploads
```

### Files disappear after deployment
**Cause**: Using local storage in production (ephemeral containers)

**Solution:**
- Set `USE_GCS=true` in production environment
- Verify auto-detection is working (check logs)

---

## Security Notes

1. **Service Account Keys**:
   - Never commit to Git
   - Use secrets management (Cloud Secret Manager, Kubernetes Secrets)
   - Rotate keys periodically

2. **File Access**:
   - Local storage: Files are publicly accessible via `/uploads/`
   - GCS: Configured with `AllUsers` reader access (line 48 of `file.go`)
   - Consider adding authentication for sensitive files

3. **File Validation**:
   - Current limit: 50MB (`ParseMultipartForm(50 << 20)`)
   - No file type validation - add as needed
   - Consider virus scanning for production

---

## Migration from Local to GCS

If you have existing files in local storage:

```bash
# Upload all files to GCS
gsutil -m cp -r ./uploads/* gs://sreeugcl/

# Update database URLs (if stored)
# This depends on your database schema
```

---

## Monitoring

### Cloud Run Logs
```bash
gcloud run logs read --service=ugcl-backend --limit=50
```

### Check Storage Usage
```bash
gsutil du -sh gs://sreeugcl
```

---

## Cost Optimization

**GCS Pricing** (as of 2024):
- Storage: ~$0.02/GB/month
- Operations: ~$0.05 per 10,000 operations
- Egress: ~$0.12/GB (to internet)

**Tips:**
- Use lifecycle policies to archive old files
- Enable compression for images
- Consider Cloud CDN for frequently accessed files
