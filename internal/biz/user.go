package biz

import (
	"context"
	"github.com/golang-jwt/jwt/v4"
	"net/http"
	"real_world/internal/conf"
	"real_world/pkg"
	myerror "real_world/pkg/error"
	"real_world/pkg/middleware/auth"

	"github.com/jinzhu/copier"

	"github.com/go-kratos/kratos/v2/log"
)

type User struct {
	Email          string
	Username       string
	Bio            string
	Image          string
	PasswordHashed string
}

type UserLogin struct {
	Email    string
	Token    string
	Username string
	Bio      string
	Image    string
}

type Profile struct {
	Username  string
	Bio       string
	Image     string
	Following bool
}

type UserRepo interface {
	Create(ctx context.Context, user *User) error
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	SaveToken(ctx context.Context, token, email, username string)
	GetToken(ctx context.Context, email, username string) (bool, string)
	Update(ctx context.Context, user *User) error
	RemoveToken(ctx context.Context, email, username string)
}

type ProfileRepo interface {
	FollowUser(ctx context.Context, followUser string, currentUser string) error
	Followed(ctx context.Context, username, currentUser string) (bool, error)
	UnFollowUser(ctx context.Context, followUser, currentUser string) error
	GetArticleAuthor(ctx context.Context, slug, username string) (*Author, error)
}

type UserUsecase struct {
	ur  UserRepo
	jwt *conf.JWT
	pr  ProfileRepo
	log *log.Helper
}

func NewUserUsecase(ur UserRepo, jwt *conf.JWT,
	pr ProfileRepo, logger log.Logger) *UserUsecase {
	return &UserUsecase{ur: ur, jwt: jwt, pr: pr, log: log.NewHelper(logger)}
}

func (uc UserUsecase) generateToken(username, email string) string {
	return auth.GenerateToken(uc.jwt.Secret, username, email)
}

func (uc UserUsecase) Register(ctx context.Context, username, email, pwd string) (*UserLogin, error) {
	u := &User{
		Email:          email,
		Username:       username,
		PasswordHashed: pkg.GeneratePasswordHash(pwd),
	}

	if err := uc.ur.Create(ctx, u); err != nil {
		return nil, err
	}

	token := uc.isTokenActivate(ctx, u)

	return &UserLogin{
		Email:    email,
		Username: username,
		Token:    token,
	}, nil
}

func (uc UserUsecase) Login(ctx context.Context, email, pwd string) (*UserLogin, error) {
	u, err := uc.ur.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if !pkg.CompareHashAndPassword(u.PasswordHashed, pwd) {
		return nil, myerror.HttpUnauthorized("password", "密码错误")
	}

	return uc.userToUserLogin(ctx, u)
}

func (uc UserUsecase) GetCurrentUser(ctx context.Context) (*UserLogin, error) {
	claims, err := jwtParse(ctx)
	if err != nil {
		return nil, err
	}
	email := claims["email"].(string)
	user, err := uc.ur.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	return uc.userToUserLogin(ctx, user)
}

// UpdateUser 更新用户信息
func (uc UserUsecase) UpdateUser(ctx context.Context, u *User, pwd string) (*UserLogin, error) {
	if len(pwd) > 0 {
		u.PasswordHashed = pkg.GeneratePasswordHash(pwd)
	}
	if err := uc.ur.Update(ctx, u); err != nil {
		return nil, err
	}

	uc.ur.RemoveToken(ctx, u.Email, u.Username)

	return uc.userToUserLogin(ctx, u)
}

func (uc UserUsecase) userToUserLogin(ctx context.Context, u *User) (*UserLogin, error) {
	var ul UserLogin
	err := copier.Copy(&ul, u)
	if err != nil {
		return nil, myerror.NewHttpError(http.StatusInternalServerError, "copier", "copy fail")
	}

	ul.Token = uc.isTokenActivate(ctx, u)
	return &ul, nil
}

func (uc UserUsecase) GetProfile(ctx context.Context, username string) (*Profile, error) {
	user, err := uc.ur.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	p := Profile{
		Username: user.Username,
		Bio:      user.Bio,
		Image:    user.Image,
	}
	claims, err := jwtParse(ctx)
	if err != nil {
		return nil, err
	}

	ok, err := uc.pr.Followed(ctx, user.Username, claims["username"].(string))
	if err != nil {
		return nil, err
	}

	if ok {
		p.Following = true
		return &p, nil
	} else {
		p.Following = false
		return &p, nil
	}
}

// FollowUser 关注用户
func (uc UserUsecase) FollowUser(ctx context.Context, followUser string) (*Profile, error) {
	claims, err := jwtParse(ctx)
	if err != nil {
		return nil, err
	}

	err = uc.pr.FollowUser(ctx, followUser, claims["username"].(string))
	if err != nil {
		return nil, err
	}

	user, err := uc.ur.GetUserByUsername(ctx, followUser)
	if err != nil {
		return nil, err
	}
	p := Profile{
		Username:  user.Username,
		Bio:       user.Bio,
		Image:     user.Image,
		Following: true,
	}

	return &p, nil
}

// UnFollowUser 关注用户
func (uc UserUsecase) UnFollowUser(ctx context.Context, followUser string) (*Profile, error) {
	claims, err := jwtParse(ctx)
	if err != nil {
		return nil, err
	}

	err = uc.pr.UnFollowUser(ctx, followUser, claims["username"].(string))
	if err != nil {
		return nil, err
	}

	user, err := uc.ur.GetUserByUsername(ctx, followUser)
	if err != nil {
		return nil, err
	}
	p := Profile{
		Username:  user.Username,
		Bio:       user.Bio,
		Image:     user.Image,
		Following: false,
	}

	return &p, nil
}

func (uc UserUsecase) GetArticleAuthor(ctx context.Context, slug string) (*Author, error) {
	claims, err := jwtParse(ctx)
	if err != nil {
		return nil, err
	}

	author, err := uc.pr.GetArticleAuthor(ctx, slug, claims["username"].(string))
	if err != nil {
		return nil, err
	}

	return author, nil
}

// isTokenActivate token是否存在, 不存在就创建并保存到redis, 返回token
func (uc UserUsecase) isTokenActivate(ctx context.Context, u *User) string {
	flag, token := uc.ur.GetToken(ctx, u.Email, u.Username)
	if flag {
		return token
	} else {
		//var sb strings.Builder
		//sb.WriteString("Token ")
		//sb.WriteString()
		token = uc.generateToken(u.Username, u.Email)
		uc.ur.SaveToken(ctx, token, u.Email, u.Username)
		return token
	}
}

// jwtParse 从jwt中解析用户信息
func jwtParse(ctx context.Context) (map[string]interface{}, error) {
	claims, ok := auth.FromContext(ctx)
	if !ok {
		return nil, myerror.HttpUnauthorized("jwt", "get fail")
	}
	return claims.(jwt.MapClaims), nil
}
