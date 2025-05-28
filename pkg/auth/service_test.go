package auth

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGenerateToken(t *testing.T) {
	Convey("Given an auth service", t, func() {
		svc := NewService()
		claims := jwt.MapClaims{"sub": "user1"}
		tok, err := svc.GenerateToken("Bearer", claims)

		Convey("Then a token is returned", func() {
			So(err, ShouldBeNil)
			So(tok.Token, ShouldNotBeEmpty)
			So(tok.RefreshToken, ShouldNotBeEmpty)
		})
	})
}

func TestAuthenticateRequest(t *testing.T) {
	Convey("Given a signed request", t, func() {
		svc := NewService()
		claims := jwt.MapClaims{"sub": "user1"}
		tok, _ := svc.GenerateToken("Bearer", claims)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+tok.Token)

		err := svc.AuthenticateRequest(req)

		Convey("Then authentication succeeds", func() {
			So(err, ShouldBeNil)
		})
	})

	Convey("Given a request without authorization header", t, func() {
		svc := NewService()
		req := httptest.NewRequest("GET", "/", nil)
		err := svc.AuthenticateRequest(req)

		Convey("Then authentication fails", func() {
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "missing authorization header")
		})
	})

	Convey("Given a request with invalid token", t, func() {
		svc := NewService()
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		err := svc.AuthenticateRequest(req)

		Convey("Then authentication fails", func() {
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "invalid token")
		})
	})
}

func TestRefreshToken(t *testing.T) {
	Convey("Given a valid refresh token", t, func() {
		svc := NewService()
		claims := jwt.MapClaims{"sub": "user1"}
		tok, _ := svc.GenerateToken("Bearer", claims)
		newTok, err := svc.RefreshToken(tok.RefreshToken)

		Convey("Then a new token is issued", func() {
			So(err, ShouldBeNil)
			So(newTok.Token, ShouldNotBeEmpty)
			So(newTok.Token, ShouldNotEqual, tok.Token)
			So(newTok.RefreshToken, ShouldNotEqual, tok.RefreshToken)
			So(newTok.Scheme, ShouldEqual, "Bearer")
		})
	})

	Convey("Given an invalid refresh token", t, func() {
		svc := NewService()
		newTok, err := svc.RefreshToken("invalid-refresh-token")

		Convey("Then refresh fails", func() {
			So(err, ShouldNotBeNil)
			So(newTok, ShouldBeNil)
			So(err.Error(), ShouldContainSubstring, "invalid refresh token")
		})
	})
}
