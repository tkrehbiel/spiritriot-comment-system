package common

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMultiError(t *testing.T) {
	t.Run("Empty MultiError", func(t *testing.T) {
		var me MultiError
		assert.Nil(t, me.Get())
	})

	t.Run("Non-empty MultiError", func(t *testing.T) {
		var me MultiError
		err1 := errors.New("first error")
		err2 := errors.New("second error")

		me.Add(err1)
		me.Add(err2)

		assert.NotNil(t, me.Get())
		assert.Equal(t, "first error; second error", me.Error())

		unwrapped := me.Unwrap()
		assert.Len(t, unwrapped, 2)
		assert.Equal(t, err1, unwrapped[0])
		assert.Equal(t, err2, unwrapped[1])
	})
}

func TestValidateReferrer(t *testing.T) {
	tests := []struct {
		name             string
		referrer         string
		allowedReferrers string
		expected         bool
	}{
		{"Empty referrer", "", "localhost,example.com", false},
		{"Allowed match", "http://localhost:1313/page", "localhost,example.com", true},
		{"Allowed match 2", "https://example.com/test", "localhost,example.com", true},
		{"Not allowed", "https://malicious.com", "localhost,example.com", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ValidateReferrer(tc.referrer, tc.allowedReferrers))
		})
	}
}

func TestGetCORSHeaders(t *testing.T) {
	headers := GetCORSHeaders("GET, POST")
	assert.Equal(t, "application/json", headers["Content-Type"])
	assert.Equal(t, "GET, POST", headers["Access-Control-Allow-Methods"])
	assert.Equal(t, "*", headers["Access-Control-Allow-Origin"])
}

func TestGetEnvVar(t *testing.T) {
	t.Run("Var exists", func(t *testing.T) {
		os.Setenv("TEST_EXISTING_VAR", "value123")
		defer os.Unsetenv("TEST_EXISTING_VAR")
		assert.Equal(t, "value123", GetEnvVar("TEST_EXISTING_VAR", "default"))
	})

	t.Run("Var missing with default", func(t *testing.T) {
		assert.Equal(t, "default", GetEnvVar("TEST_NON_EXISTENT_VAR", "default"))
	})
}

func TestGetEnvVar_Fatal(t *testing.T) {
	if os.Getenv("BE_CRASHER") == "1" {
		GetEnvVar("TEST_NON_EXISTENT_FATAL_VAR", "")
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestGetEnvVar_Fatal")
	cmd.Env = append(os.Environ(), "BE_CRASHER=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

func TestLoadAWSConfig(t *testing.T) {
	ctx := context.TODO()
	cfg, err := LoadAWSConfig(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "us-east-1", cfg.Region)
}
