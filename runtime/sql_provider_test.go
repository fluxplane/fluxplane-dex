package runtime

import "testing"

func TestSQLProviderTargetFromCredentialsUsesEndpointURLWithSecretCredentials(t *testing.T) {
	creds := sqlProviderCredentials{
		Username: "app_user",
		Password: "app_pass",
	}
	target, ok, err := sqlProviderTargetFromCredentials(creds, "", "app", "mysql://db.example.com:3307")
	if err != nil {
		t.Fatalf("sqlProviderTargetFromCredentials: %v", err)
	}
	if !ok {
		t.Fatal("sqlProviderTargetFromCredentials did not build a target")
	}
	if target.Driver != "mysql" || target.Database != "app" {
		t.Fatalf("target = %#v", target)
	}
	if target.DSN != "app_user:app_pass@tcp(db.example.com:3307)/app?parseTime=true" {
		t.Fatalf("DSN = %q", target.DSN)
	}
	if target.SafeURL != "mysql://app_user:xxxxx@db.example.com:3307/app" {
		t.Fatalf("SafeURL = %q", target.SafeURL)
	}
}

func TestSQLProviderCredentialsFromDataReadsCrossplaneSecretKeys(t *testing.T) {
	creds := sqlProviderCredentialsFromData(map[string]string{
		"username": "app_user",
		"password": "app_pass",
		"endpoint": "db.example.com",
		"port":     "3307",
	})
	if creds.Username != "app_user" || creds.Password != "app_pass" || creds.Host != "db.example.com" || creds.Port != "3307" {
		t.Fatalf("creds = %#v", creds)
	}
}
