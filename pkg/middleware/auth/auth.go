package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	myerror "real_world/pkg/error"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/golang-jwt/jwt/v4"
)

// 描述: jwt
// 作者: hgy
// 创建日期: 2022/11/27

const (
	jwtPrefix = "Token"
	jwtHeader = "Authorization"
)

type authKey struct{}

// JWTAuth 校验JWT
func JWTAuth(secret string) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		var newCtx context.Context
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				tokenString := tr.RequestHeader().Get(jwtHeader)
				auths := strings.SplitN(tokenString, " ", 2)
				if len(auths) != 2 || !strings.EqualFold(auths[0], jwtPrefix) {
					return nil, fmt.Errorf("jwt token missing")
				}

				token, err := jwt.Parse(auths[1], func(token *jwt.Token) (interface{}, error) {
					// Don't forget to validate the alg is what you expect:
					if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"]) // TODO 错误处理
					}
					// hmacSampleSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
					return []byte(secret), nil
				})

				if err != nil {
					return nil, myerror.NewHttpError(http.StatusUnauthorized, "JWT_PARSE_ERROR", "no authorization")
				}

				if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
					//var info [2]string
					//info[0] = claims["username"].(string)
					//info[1] = claims["email"].(string)
					//spew.Dump(claims["username"], claims["email"])
					newCtx = NewContext(ctx, claims)
				} else {
					return nil, myerror.NewHttpError(http.StatusUnauthorized, "JWT_PARSE_ERROR", "no authorization")
				}
			}
			return handler(newCtx, req)
		}
	}
}

func GenerateToken(secret, username, email string) string {
	// Create a new token object, specifying signing method and the claims
	// you would like it to contain.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email":    email,
		"username": username,
		"nbf":      time.Date(2015, 10, 10, 12, 0, 0, 0, time.UTC).Unix(),
	})

	// Sign and get the complete encoded token as a string using the secret
	// 参数key为[]byte数组
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		panic(err)
	}
	return tokenString
}

// NewContext put auth info into context
func NewContext(ctx context.Context, info jwt.Claims) context.Context {
	return context.WithValue(ctx, authKey{}, info)
}

// FromContext extract auth info from context
func FromContext(ctx context.Context) (token jwt.Claims, ok bool) {
	token, ok = ctx.Value(authKey{}).(jwt.Claims)
	return
}
