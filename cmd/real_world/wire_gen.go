// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package main

import (
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"real_world/internal/biz"
	"real_world/internal/conf"
	"real_world/internal/data"
	"real_world/internal/server"
	"real_world/internal/service"
)

import (
	_ "go.uber.org/automaxprocs"
)

// Injectors from wire.go:

// wireApp init kratos application.
func wireApp(confServer *conf.Server, confData *conf.Data, jwt *conf.JWT, logger log.Logger) (*kratos.App, func(), error) {
	db := data.NewDB(confData, logger)
	client := data.NewRedis(confData, logger)
	dataData, cleanup, err := data.NewData(confData, logger, db, client)
	if err != nil {
		return nil, nil, err
	}
	userRepo := data.NewUserRepo(dataData, logger)
	profileRepo := data.NewProfileRepo(dataData, logger)
	userUsecase := biz.NewUserUsecase(userRepo, jwt, profileRepo, logger)
	articleRepo := data.NewArticleRepo(dataData, logger)
	commentRepo := data.NewCommentRepo(dataData, logger)
	tagRepo := data.NewTagRepo(dataData, logger)
	articleUsecase := biz.NewArticleUsecase(articleRepo, commentRepo, tagRepo, logger)
	realWorldService := service.NewRealWorldService(userUsecase, articleUsecase)
	grpcServer := server.NewGRPCServer(confServer, realWorldService, logger)
	httpServer := server.NewHTTPServer(confServer, realWorldService, logger, jwt)
	app := newApp(logger, grpcServer, httpServer)
	return app, func() {
		cleanup()
	}, nil
}
