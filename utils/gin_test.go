package utils

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetQueryBool(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{name: "missing", want: false},
		{name: "false", query: "?wait=false", want: false},
		{name: "true", query: "?wait=true", want: true},
		{name: "empty value", query: "?wait", want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = httptest.NewRequest("GET", "/"+test.query, nil)

			if got := GetQueryBool(context, "wait"); got != test.want {
				t.Fatalf("GetQueryBool() = %t, want %t", got, test.want)
			}
		})
	}
}
