package helper

import (
	"fmt"
	"reflect"

	"github.com/dgrijalva/jwt-go"
)

func (dv *driverHelper) ParseJWTclaims(jwtString string) (jwt.MapClaims, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(jwtString, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("error parsing rhm pull secret: %s", err.Error())
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("error retrieving claims from rhm pull secret")
	}
	return claims, nil
}

func (dv *driverHelper) GetClaimValue(claims jwt.MapClaims, key string) (string, error) {
	valGeneric, ok := claims[key]
	if !ok {
		return "", fmt.Errorf("error retrieving %s from claims in rhm pull secret", key)
	}
	keyType := reflect.TypeOf(valGeneric)
	if keyType.String() != "string" {
		return "", fmt.Errorf("exepcted string type for %s, but has type %s in rhm pull secret", key, keyType.String())
	}
	return valGeneric.(string), nil
}
