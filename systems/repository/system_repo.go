package repository

import (
	"context"
	"systems/models"

	"cloud.google.com/go/firestore"
)

type SystemRepository struct {
	client *firestore.Client
}

func NewSystemRepository(client *firestore.Client) *SystemRepository {
	return &SystemRepository{
		client: client,
	}
}

func (r *SystemRepository) GetSystem(ctx context.Context) ([]models.System, error) {
	responseList := make([]models.System, 0)

	return responseList, nil
}
