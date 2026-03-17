package models

import "imposter-api/db"

type Review struct {
	ID          int    `json:"id"`
	Description string `json:"description" binding:"required"`
	Stars       int    `json:"stars" binding:"required,min=1,max=5"`
}

func CreateReview(r *Review) error {
	query := `INSERT INTO reviews (description, stars) VALUES ($1, $2) RETURNING id`
	return db.DB.QueryRow(query, r.Description, r.Stars).Scan(&r.ID)
}
