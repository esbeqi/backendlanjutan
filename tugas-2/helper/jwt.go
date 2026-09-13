package helper

import (
	"errors"
	"strconv"
	"time"

	"api-students/app/model"

	"github.com/golang-jwt/jwt/v5"
)

type JWTManager struct {
	Secret    []byte
	Issuer    string
	AccessTTL time.Duration
}

func NewJWTManager(secret, issuer string, accessTTL time.Duration) *JWTManager {
	return &JWTManager{
		Secret:    []byte(secret),
		Issuer:    issuer,
		AccessTTL: accessTTL,
	}
}

func (m *JWTManager) GenerateAccessToken(user model.User) (string, error) {
	now := time.Now()
	expiresAt := now.Add(m.AccessTTL)

	claims := jwt.MapClaims{
		"sub":      strconv.Itoa(user.ID),
		"username": user.Username,
		"role":     user.Role,
		"iss":      m.Issuer,
		"iat":      now.Unix(),
		"exp":      expiresAt.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(m.Secret)
}

func (m *JWTManager) ParseAccessToken(tokenString string) (model.AuthUser, error) {
	token, err := jwt.Parse(
		tokenString,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("algoritma JWT tidak valid")
			}

			return m.Secret, nil
		},
		jwt.WithIssuer(m.Issuer),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		return model.AuthUser{}, err
	}

	if !token.Valid {
		return model.AuthUser{}, errors.New("token tidak valid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return model.AuthUser{}, errors.New("claims tidak valid")
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return model.AuthUser{}, errors.New("subject tidak valid")
	}

	userID, err := strconv.Atoi(sub)
	if err != nil {
		return model.AuthUser{}, errors.New("subject tidak valid")
	}

	username, ok := claims["username"].(string)
	if !ok {
		return model.AuthUser{}, errors.New("username tidak valid")
	}

	role, ok := claims["role"].(string)
	if !ok {
		return model.AuthUser{}, errors.New("role tidak valid")
	}

	return model.AuthUser{
		UserID:   userID,
		Username: username,
		Role:     role,
	}, nil
}
