package model

type Tag struct {
	Name *string `db:"name" json:"name"`
}

type Tags struct {
	ID   int64  `db:"id" json:"tag_id"`
	Name string `db:"name" json:"name,omitempty"`
}

type TagInsert struct {
	CallRowID int64  `json:"call_row_id"`
	TagID     *int64 `json:"tag_id,omitempty"`
}
