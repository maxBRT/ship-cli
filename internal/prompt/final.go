package prompt

// Final returns the built-in Final phase prompt.
// Ship verifies that Final opens a pull request.
func Final(in FinalInput) string {
	return render("final.md", in)
}
