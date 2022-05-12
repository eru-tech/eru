package jwt

import (
	"context"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/lestrrat-go/jwx/jwk"
	"log"
)

func FetchKeyJWK(kid string, jwkurl string) (interface{}, error) {
	// TODO Use jwk.AutoRefresh if you intend to keep reuse the JWKS over and over
	set, err := jwk.Fetch(context.Background(), jwkurl)
	if err != nil {
		log.Printf("failed to parse JWK: %s", err)
		return nil, errors.New("Failed to parse JWK")
	}

	// Key sets can be serialized back to JSON
	/*
		{
			jsonbuf, err := json.Marshal(set)
			if err != nil {
				log.Printf("failed to marshal key set into JSON: %s", err)
				return nil,errors.New(fmt.Sprint("Failed to marshal key set into JSON - ",err.Error()))
			}
		}

	*/

	for it := set.Iterate(context.Background()); it.Next(context.Background()); {

		pair := it.Pair()
		key := pair.Value.(jwk.Key)
		log.Println("key.Algorithm() = ", key.Algorithm())
		log.Println("key.KeyID() = ", key.KeyID())
		log.Println("kid = ", kid)
		if kid == key.KeyID() {
			var rawkey interface{} // This is the raw key, like *rsa.PrivateKey or *ecdsa.PrivateKey
			if err := key.Raw(&rawkey); err != nil {
				log.Printf("failed to create public key: %s", err)
				return nil, errors.New(fmt.Sprint("Failed to create public key - ", err.Error()))
			}
			// Use rawkey for jws.Verify() or whatever.
			return rawkey, nil
			/*
				// You can create jwk.Key from a raw key, too
				fromRawKey, err := jwk.New(rawkey)
				if err != nil {
					log.Printf("failed to acquire raw key from jwk.Key: %s", err)
					return nil
				}

				// Keys can be serialized back to JSON
				jsonbuf, err := json.Marshal(key)
				if err != nil {
					log.Printf("failed to marshal key into JSON: %s", err)
					return nil
				}
				log.Printf("serialized back to JSON")
				log.Printf("%s", jsonbuf)

				// If you know the underlying Key type (RSA, EC, Symmetric), you can
				// create an empty instance first
				//    key := jwk.NewRSAPrivateKey()
				// ..and then use json.Unmarshal
				//    json.Unmarshal(key, jsonbuf)
				//
				// but if you don't know the type first, you have an abstract type
				// jwk.Key, which can't be used as the first argument to json.Unmarshal
				//
				// In this case, use jwk.Parse()
				fromJSONKey, err := jwk.Parse(jsonbuf)
				if err != nil {
					log.Printf("failed to parse json: %s", err)
					return nil
				}
				_ = fromJSONKey
				_ = fromRawKey

			*/
		}
	}
	// OUTPUT:
	return nil, errors.New(fmt.Sprint("kid ", kid, " not found"))
}

func DecryptTokenJWK(strToken string, jwkurl string) (objToken interface{}, err error) {
	log.Println(strToken)
	log.Println(jwkurl)

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
		log.Println("token.Header ============== ")
		log.Println(token.Header)
		key, err := FetchKeyJWK(token.Header["kid"].(string), jwkurl)
		return key, err
	})

	if err != nil {
		log.Println(err)
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
		log.Println("Claim from request token")
		log.Println(map[string]interface{}{"claims": claims, "type": "auth"})
		log.Println(obj)
		return obj, nil
	}
	return nil, errors.New("AUTH: JWT token could not be verified")
}
