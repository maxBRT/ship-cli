package prompt

// Review returns the built-in Review phase prompt.
// Ship allows Review to succeed without a new commit.
func Review(in ReviewInput) string {
	return render("review.md", in)
}
