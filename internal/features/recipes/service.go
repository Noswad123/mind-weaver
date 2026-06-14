package recipes

import (
	"context"

	"github.com/Noswad123/mind-weaver/internal/infra/db"
)

type Store interface {
	ListRecipes(ctx context.Context, scopeDomains []string) ([]db.RecipeRow, error)
	ListIngredients(ctx context.Context) ([]db.IngredientRow, error)
	ListRecipeIngredientMentions(ctx context.Context, unresolvedOnly bool) ([]db.RecipeIngredientMentionRow, error)
}

type Service struct{ store Store }

func New(store Store) *Service { return &Service{store: store} }

func (s *Service) ListRecipes(ctx context.Context, scopeDomains []string) ([]db.RecipeRow, error) {
	return s.store.ListRecipes(ctx, scopeDomains)
}

func (s *Service) ListIngredients(ctx context.Context) ([]db.IngredientRow, error) {
	return s.store.ListIngredients(ctx)
}

func (s *Service) ListIngredientMentions(ctx context.Context, unresolvedOnly bool) ([]db.RecipeIngredientMentionRow, error) {
	return s.store.ListRecipeIngredientMentions(ctx, unresolvedOnly)
}
