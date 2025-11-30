package main

// THIS CODE WILL BE UPDATED WITH SCHEMA CHANGES. PREVIOUS IMPLEMENTATION FOR SCHEMA CHANGES WILL BE KEPT IN THE COMMENT SECTION. IMPLEMENTATION FOR UNCHANGED SCHEMA WILL BE KEPT.

import (
	"context"

	"github.com/99designs/gqlgen/generated"
)

type Resolver struct{}

// Hello is the resolver for the hello field.
func (r *queryResolver) Hello(ctx context.Context) (string, error) {
	panic("not implemented")
}

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type queryResolver struct{ *Resolver }

// !!! WARNING !!!
// The code below was going to be deleted when updating resolvers. It has been copied here so you have
// one last chance to move it out of harms way if you want. There are two reasons this happens:
//  - When renaming or deleting a resolver the old code will be put in here. You can safely delete
//    it when you're done.
//  - You have helper methods in this file. Move them out to keep these resolver files clean.
/*
	type Resolver struct{}
func (r *mutationResolver) CreateUser(ctx context.Context, name string, email string) (*generated.User, error) {
	panic("not implemented")
}
func (r *mutationResolver) CreatePost(ctx context.Context, title string, content string, authorID string) (*generated.Post, error) {
	panic("not implemented")
}
func (r *queryResolver) User(ctx context.Context, id string) (*generated.User, error) {
	panic("not implemented")
}
func (r *queryResolver) Users(ctx context.Context) ([]*generated.User, error) {
	panic("not implemented")
}
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }
type mutationResolver struct{ *Resolver }
*/
