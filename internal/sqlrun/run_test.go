package sqlrun

import (
	"testing"
)

func TestIsReadOnlySQL_Select(t *testing.T) {
	tests := []string{
		"SELECT * FROM users",
		"select id, name from users where active = true",
		"  SELECT count(*) FROM orders  ",
		"WITH cte AS (SELECT * FROM t) SELECT * FROM cte",
	}
	for _, sql := range tests {
		if err := IsReadOnlySQL(sql); err != nil {
			t.Errorf("IsReadOnlySQL(%q) = %v, want nil", sql, err)
		}
	}
}

func TestIsReadOnlySQL_Insert(t *testing.T) {
	if err := IsReadOnlySQL("INSERT INTO users (name) VALUES ('test')"); err == nil {
		t.Error("IsReadOnlySQL(INSERT) should return error, got nil")
	}
}

func TestIsReadOnlySQL_Drop(t *testing.T) {
	if err := IsReadOnlySQL("DROP TABLE users"); err == nil {
		t.Error("IsReadOnlySQL(DROP) should return error, got nil")
	}
}

func TestIsReadOnlySQL_Update(t *testing.T) {
	if err := IsReadOnlySQL("UPDATE users SET name = 'x' WHERE id = 1"); err == nil {
		t.Error("IsReadOnlySQL(UPDATE) should return error, got nil")
	}
}

func TestIsReadOnlySQL_Delete(t *testing.T) {
	if err := IsReadOnlySQL("DELETE FROM users WHERE id = 1"); err == nil {
		t.Error("IsReadOnlySQL(DELETE) should return error, got nil")
	}
}

func TestIsReadOnlySQL_Create(t *testing.T) {
	if err := IsReadOnlySQL("CREATE TABLE foo (id int)"); err == nil {
		t.Error("IsReadOnlySQL(CREATE) should return error, got nil")
	}
}

func TestIsReadOnlySQL_Alter(t *testing.T) {
	if err := IsReadOnlySQL("ALTER TABLE users ADD COLUMN age int"); err == nil {
		t.Error("IsReadOnlySQL(ALTER) should return error, got nil")
	}
}

func TestIsReadOnlySQL_Truncate(t *testing.T) {
	if err := IsReadOnlySQL("TRUNCATE TABLE users"); err == nil {
		t.Error("IsReadOnlySQL(TRUNCATE) should return error, got nil")
	}
}
