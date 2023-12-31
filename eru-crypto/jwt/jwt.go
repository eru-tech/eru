package jwt

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/golang-jwt/jwt/v4"
	"github.com/lestrrat-go/jwx/jwk"
)

func FetchKeyJWK(ctx context.Context, kid string, jwkurl string) (interface{}, error) {
	// TODO Use jwk.AutoRefresh if you intend to keep reuse the JWKS over and over
	set, err := jwk.Fetch(ctx, jwkurl)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("failed to parse JWK: %s", err))
		return nil, errors.New("Failed to parse JWK")
	}

	for it := set.Iterate(context.Background()); it.Next(context.Background()); {

		pair := it.Pair()
		key := pair.Value.(jwk.Key)
		if kid == key.KeyID() {
			var rawkey interface{} // This is the raw key, like *rsa.PrivateKey or *ecdsa.PrivateKey
			if err := key.Raw(&rawkey); err != nil {
				err = errors.New(fmt.Sprint("Failed to create public key - ", err.Error()))
				logs.WithContext(ctx).Error(err.Error())
				return nil, err
			}
			// Use rawkey for jws.Verify() or whatever.
			return rawkey, nil

		}
	}
	// OUTPUT:
	err = errors.New(fmt.Sprint("kid ", kid, " not found"))
	logs.WithContext(ctx).Error(err.Error())
	return nil, err
}

func DecryptTokenJWK(ctx context.Context, strToken string, jwkurl string) (objToken interface{}, err error) {

	tokenObj, err := jwt.Parse(strToken, func(token *jwt.Token) (interface{}, error) {
		// TODO Don't forget to validate the alg is what you expect:

		//if token.Method.Alg() != string(secret.Alg) {
		//	return nil, fmt.Errorf("invalid token algorithm provided wanted (%s) got (%s)", secret.Alg, token.Method.Alg())
		//}
		//
		//if secret.JwkURL != "" {
		//	return secret.JwkKey, nil
		//}

		//switch secret.Alg {
		//case config.RS256:
		//	return jwt.ParseRSAPublicKeyFromPEM([]byte(secret.PublicKey))
		//case config.HS256, "":
		//		return []byte(secret.Secret), nil
		//	default:
		//		return nil, fmt.Errorf("invalid token algorithm (%s) provided", secret.Alg)
		//	}
		key, err := FetchKeyJWK(ctx, token.Header["kid"].(string), jwkurl)
		return key, err
	})

	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	//_=tokenObj

	// Get the claims
	if claims, ok := tokenObj.Claims.(jwt.MapClaims); ok && tokenObj.Valid {
		if err := claims.Valid(); err != nil {
			return nil, err
		}
		obj := make(map[string]interface{}, len(claims))
		for key, val := range claims {
			obj[key] = val
		}

		/*
			if len(secret.Issuer) > 0 {
				c, ok := claims["iss"]
				if !ok {
					return nil, errors.New("claim (iss) not provided in token")
				}
				if err := verifyClaims(c, secret.Issuer); err != nil {
					return nil, err
				}
			}

			if len(secret.Audience) > 0 {
				c, ok := claims["aud"]
				if !ok {
					return nil, errors.New("claim (aud) not provided in token")
				}
				if err := verifyClaims(c, secret.Audience); err != nil {
					return nil, err
				}
			}
		*/
		return obj, nil
	}
	err = errors.New("AUTH: JWT token could not be verified")
	logs.WithContext(ctx).Error(err.Error())
	return nil, err
}
func CreateJWT(ctx context.Context, privateKeyStr string, claimsMap map[string]interface{}) (tokenString string, err error) {
	logs.WithContext(ctx).Info(privateKeyStr)
	token := jwt.New(jwt.SigningMethodRS256)

	var privateKey *rsa.PrivateKey

	if privateKey, err = jwt.ParseRSAPrivateKeyFromPEM([]byte(privateKeyStr)); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	claims := token.Claims.(jwt.MapClaims)
	for k, v := range claimsMap {
		claims[k] = v
	}
	tokenString, err = token.SignedString(privateKey)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
	}
	return
}
