package diff

// Comment is a review annotation attached to a specific line.
type Comment struct {
	File string
	Line int
	Side string // "old" | "new"
	Body string
}
