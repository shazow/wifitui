package tui

// InnerWidth returns fallbackWidth minus frame, clamped to min.
// Use this when you already know the total horizontal frame cost
// (for example border + padding + margin) and need a safe content width.
func InnerWidth(fallbackWidth int, frame int, min int) int {
	inner := fallbackWidth - frame
	if inner < min {
		return min
	}
	return inner
}

// InnerHeight returns fallbackHeight minus frame, clamped to min.
// Use this for vertical layout where parent decorations (margins, borders,
// status/help rows) should be removed from a fallback window height.
func InnerHeight(fallbackHeight int, frame int, min int) int {
	inner := fallbackHeight - frame
	if inner < min {
		return min
	}
	return inner
}

// BoxContentWidth returns usable inner width for a boxed region by subtracting
// horizontalMargin and borderAndPadding from fallbackWidth, clamped to min.
// Use this when your layout has a margin outside a bordered/padded box.
func BoxContentWidth(fallbackWidth int, horizontalMargin int, borderAndPadding int, min int) int {
	return InnerWidth(fallbackWidth, horizontalMargin+borderAndPadding, min)
}
