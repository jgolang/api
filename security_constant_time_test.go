package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// The tests in this file pin how the default basic validator compares
// credentials. A compare with == or a short circuit returns the same results,
// so no behavioral test can tell it apart from a constant-time one; they read
// the source of security.go instead and fail if the structure changes.

// TestValidateCredentialsComparesBothFieldsWithoutShortCircuit checks that
// validateCredentials compares the username and the password with
// constantTimeEqual, each in an unconditional statement of its body, so both
// compares always run, and never compares them another way.
func TestValidateCredentialsComparesBothFieldsWithoutShortCircuit(t *testing.T) {
	body := parseSecurityFunc(t, "validateCredentials").Body

	// The body is pinned as: the credentialsConfigured guard, the two
	// compares, one return. Any statement in between (an early return after
	// the username compare) changes the length or the shape.
	if len(body.List) != 4 {
		t.Fatalf("validateCredentials has %d statements, want 4: guard, two compares, return", len(body.List))
	}
	if _, ok := body.List[0].(*ast.IfStmt); !ok {
		t.Fatalf("validateCredentials statement 1 is %T, want the credentialsConfigured guard", body.List[0])
	}
	var compared [][2]string
	for _, statement := range body.List[1:3] {
		if call := constantTimeEqualCall(statement); call != nil {
			compared = append(compared, [2]string{identName(call.Args[0]), identName(call.Args[1])})
		}
	}
	want := [][2]string{{"username", "Username"}, {"password", "Password"}}
	if len(compared) != len(want) || compared[0] != want[0] || compared[1] != want[1] {
		t.Fatalf("constantTimeEqual compares in statements 2 and 3 = %q, want %q", compared, want)
	}
	if _, ok := body.List[3].(*ast.ReturnStmt); !ok {
		t.Fatalf("validateCredentials statement 4 is %T, want the return", body.List[3])
	}

	calls := 0
	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.CallExpr:
			if identName(typed.Fun) == "constantTimeEqual" {
				calls++
				return false
			}
		case *ast.BinaryExpr:
			if typed.Op == token.LAND || typed.Op == token.LOR {
				t.Errorf("validateCredentials uses %s, which can skip a compare", typed.Op)
			}
		case *ast.Ident:
			switch typed.Name {
			case "username", "password", "Username", "Password":
				t.Errorf("validateCredentials uses %s outside constantTimeEqual", typed.Name)
			}
		}
		return true
	})
	if calls != len(want) {
		t.Fatalf("validateCredentials calls constantTimeEqual %d times, want %d", calls, len(want))
	}
}

// TestConstantTimeEqualHashesAndUsesSubtle checks that constantTimeEqual
// hashes both values, so they have the same length, and compares the sums
// with subtle.ConstantTimeCompare alone.
func TestConstantTimeEqualHashesAndUsesSubtle(t *testing.T) {
	body := parseSecurityFunc(t, "constantTimeEqual").Body

	// Pinned as: two hashes, then return the subtle compare, so no other
	// compare (a switch on the sums) can run first.
	if len(body.List) != 3 {
		t.Fatalf("constantTimeEqual has %d statements, want 3: two hashes and the return", len(body.List))
	}
	result, ok := body.List[2].(*ast.ReturnStmt)
	if !ok || len(result.Results) != 1 {
		t.Fatalf("constantTimeEqual statement 3 is %T, want return subtle.ConstantTimeCompare(...)", body.List[2])
	}
	if call, ok := result.Results[0].(*ast.CallExpr); !ok || selectorName(call.Fun) != "subtle.ConstantTimeCompare" {
		t.Fatalf("constantTimeEqual returns %T, want the subtle.ConstantTimeCompare result", result.Results[0])
	}

	calls := map[string]int{}
	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.CallExpr:
			name := selectorName(typed.Fun)
			calls[name]++
			if name == "sha256.Sum256" {
				return false
			}
		case *ast.BinaryExpr:
			if typed.Op == token.EQL || typed.Op == token.NEQ {
				t.Errorf("constantTimeEqual compares with %s", typed.Op)
			}
		case *ast.Ident:
			if typed.Name == "got" || typed.Name == "want" {
				t.Errorf("constantTimeEqual uses %s outside sha256.Sum256", typed.Name)
			}
		}
		return true
	})
	if calls["sha256.Sum256"] != 2 || calls["subtle.ConstantTimeCompare"] != 1 || len(calls) != 2 {
		t.Fatalf("constantTimeEqual calls %v, want sha256.Sum256 twice and subtle.ConstantTimeCompare once", calls)
	}
}

func parseSecurityFunc(t *testing.T, name string) *ast.FuncDecl {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "security.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing security.go: %v", err)
	}
	for _, declaration := range file.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok && function.Recv == nil && function.Name.Name == name {
			return function
		}
	}
	t.Fatalf("security.go has no func %s", name)
	return nil
}

// constantTimeEqualCall returns the constantTimeEqual call that statement
// assigns, or nil.
func constantTimeEqualCall(statement ast.Stmt) *ast.CallExpr {
	assign, ok := statement.(*ast.AssignStmt)
	if !ok || len(assign.Rhs) != 1 {
		return nil
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok || identName(call.Fun) != "constantTimeEqual" || len(call.Args) != 2 {
		return nil
	}
	return call
}

func identName(expression ast.Expr) string {
	if ident, ok := expression.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// selectorName returns "pkg.Name" for a qualified call, or the bare name.
func selectorName(expression ast.Expr) string {
	if selector, ok := expression.(*ast.SelectorExpr); ok {
		return identName(selector.X) + "." + selector.Sel.Name
	}
	return identName(expression)
}
