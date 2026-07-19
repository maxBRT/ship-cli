package prompt

// Implement returns the built-in Implement phase prompt.
// Ship verifies that this phase produces commit(s).
func Implement(in ImplementInput) string {
	return render("implement.md", in)
}
