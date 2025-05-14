package model

type Vendor struct {
	ID   *int    `db:"id" json:"id,omitempty"`
	Name *string `db:"name" json:"name"`
}

type VendorSimple struct {
	Name *string `json:"name"`
}

type SwaggerVendorList struct {
	Status string   `json:"status"`
	Data   []Vendor `json:"data"`
}
