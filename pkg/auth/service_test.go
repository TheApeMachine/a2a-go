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
}

func TestRefreshToken(t *testing.T) {
	Convey("Given a valid refresh token", t, func() {
		svc := NewService()
		claims := jwt.MapClaims{"sub": "user1"}
		tok, _ := svc.GenerateToken("Bearer", claims)
		time.Sleep(10 * time.Millisecond)
		newTok, err := svc.RefreshToken(tok.RefreshToken)

		Convey("Then a new token is issued", func() {
			So(err, ShouldBeNil)
			So(newTok.Token, ShouldNotBeEmpty)
		})
	})
}
