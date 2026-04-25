package main

import (
	"fmt"
	"reflect"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

func main() {
	config.Connect()

	policy := models.Policy{
		Name:        "dbg_policy_dryrun",
		DisplayName: "Debug Policy",
		Description: "debug",
		Effect:      models.PolicyEffectAllow,
		Priority:    1,
		Status:      models.PolicyStatusDraft,
		Conditions: models.JSONMap{
			"attribute": "user.department",
			"operator":  "EQUALS",
			"value":     "Engineering",
		},
		Actions:   models.JSONArray{"project:read"},
		Resources: models.JSONArray{"project"},
		Metadata:  models.JSONMap{},
		CreatedBy: uuid.MustParse("4f74d110-db88-4c7d-b5b5-ccf5ace1c0ea"),
	}

	dryRun := config.DB.Session(&gorm.Session{DryRun: true}).Create(&policy)
	fmt.Println("SQL:")
	fmt.Println(dryRun.Statement.SQL.String())
	fmt.Println("VARS:")
	for i, value := range dryRun.Statement.Vars {
		fmt.Printf("%d: %T => %#v\n", i, value, value)
	}

	fmt.Println("VALUER OUTPUT:")
	if value, err := policy.Conditions.Value(); err == nil {
		fmt.Printf("conditions: %T => %#v\n", value, value)
	} else {
		fmt.Println("conditions err:", err)
	}
	if value, err := policy.Actions.Value(); err == nil {
		fmt.Printf("actions: %T => %#v\n", value, value)
	} else {
		fmt.Println("actions err:", err)
	}
	if value, err := policy.Resources.Value(); err == nil {
		fmt.Printf("resources: %T => %#v\n", value, value)
	} else {
		fmt.Println("resources err:", err)
	}
	if value, err := policy.Metadata.Value(); err == nil {
		fmt.Printf("metadata: %T => %#v\n", value, value)
	} else {
		fmt.Println("metadata err:", err)
	}

	fmt.Println("TYPE CHECK:")
	fmt.Println(reflect.TypeOf(policy.Conditions), reflect.TypeOf(policy.Actions), reflect.TypeOf(policy.Resources), reflect.TypeOf(policy.Metadata))
}
