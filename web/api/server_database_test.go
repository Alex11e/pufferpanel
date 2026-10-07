package api

import (
	"testing"

	"github.com/pufferpanel/pufferpanel/v3/models"
)

func TestValidateDatabaseVariableMapping(t *testing.T) {
	valid := map[string]string{
		"host":     "DB_HOST",
		"port":     "DB_PORT",
		"database": "DB_DATABASE",
		"username": "DB_USERNAME",
		"password": "DB_PASSWORD",
	}
	tests := []struct {
		name    string
		mapping map[string]string
		wantErr bool
	}{
		{name: "standard mapping", mapping: valid},
		{name: "missing role", mapping: map[string]string{"host": "DB_HOST"}, wantErr: true},
		{name: "duplicate variable", mapping: map[string]string{"host": "DB_VALUE", "port": "DB_VALUE", "database": "DB_DATABASE", "username": "DB_USERNAME", "password": "DB_PASSWORD"}, wantErr: true},
		{name: "invalid variable name", mapping: map[string]string{"host": "DB-HOST", "port": "DB_PORT", "database": "DB_DATABASE", "username": "DB_USERNAME", "password": "DB_PASSWORD"}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateDatabaseVariableMapping(test.mapping); (err != nil) != test.wantErr {
				t.Fatalf("validateDatabaseVariableMapping() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

func TestDatabaseServersShareNode(t *testing.T) {
	tests := []struct {
		name         string
		parentNodeID uint
		databaseNode uint
		want         bool
	}{
		{name: "same remote node", parentNodeID: 4, databaseNode: 4, want: true},
		{name: "different nodes", parentNodeID: 4, databaseNode: 5, want: false},
		{name: "local node", want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parent := &models.Server{Node: models.Node{ID: test.parentNodeID}}
			databaseServer := &models.Server{Node: models.Node{ID: test.databaseNode}}
			if got := databaseServersShareNode(parent, databaseServer); got != test.want {
				t.Fatalf("databaseServersShareNode() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestValidateDatabaseHost(t *testing.T) {
	tests := []struct {
		host    string
		wantErr bool
	}{
		{host: "192.168.1.20"},
		{host: "db.example.com"},
		{host: "database host", wantErr: true},
		{host: "", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.host, func(t *testing.T) {
			if err := validateDatabaseHost(test.host); (err != nil) != test.wantErr {
				t.Fatalf("validateDatabaseHost(%q) error = %v, wantErr %t", test.host, err, test.wantErr)
			}
		})
	}
}
