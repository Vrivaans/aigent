package database

import (
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
)

func currentEnvironment() string {
	env := strings.TrimSpace(os.Getenv("AIGENT_ENV"))
	if env == "" {
		return "*"
	}
	return env
}

func matchEnvironment(policyEnv, runtimeEnv string) bool {
	policyEnv = strings.TrimSpace(policyEnv)
	if policyEnv == "" || policyEnv == "*" {
		return true
	}
	return policyEnv == runtimeEnv
}

// MatchToolPattern reports whether toolName matches a glob pattern (e.g. odoo_*).
func MatchToolPattern(pattern, toolName string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || pattern == "*" {
		return true
	}
	matched, err := filepath.Match(pattern, toolName)
	return err == nil && matched
}

// ToolMatchesApprovalPolicy returns true when an active policy requires approval for the tool.
func ToolMatchesApprovalPolicy(db *gorm.DB, toolName string) (bool, error) {
	if db == nil {
		return false, nil
	}
	var policies []ApprovalPolicy
	if err := db.Order("id asc").Find(&policies).Error; err != nil {
		return false, err
	}
	runtimeEnv := currentEnvironment()
	for _, p := range policies {
		if !p.RequiresApproval {
			continue
		}
		if !matchEnvironment(p.Environment, runtimeEnv) {
			continue
		}
		if MatchToolPattern(p.ToolPattern, toolName) {
			return true, nil
		}
	}
	return false, nil
}

// ListApprovalPolicies returns all policies ordered by id.
func ListApprovalPolicies(db *gorm.DB) ([]ApprovalPolicy, error) {
	var policies []ApprovalPolicy
	if err := db.Order("id asc").Find(&policies).Error; err != nil {
		return nil, err
	}
	return policies, nil
}
