package auth

import (
	"crypto/rand"
	"fmt"
	"log"
	"sync"
	"time"

	"koishi/chameleon/homego/internal/config"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

type VisitRecord struct {
	Index string
	Time  int64
}

type AuthService struct {
	config        *config.Config
	visits        map[string][]VisitRecord
	authedClients []string
	jwtSecret     []byte
	mu            sync.RWMutex
}

const (
	totpCodeLength = 6
)

func NewAuthService(cfg *config.Config) *AuthService {
	secret, err := genJWTSecret()
	if err != nil {
		log.Fatalf("Failed to generate JWT: %v", err)
		panic(err)
	}

	return &AuthService{
		config:    cfg,
		visits:    make(map[string][]VisitRecord),
		jwtSecret: secret,
	}
}

func genJWTSecret() ([]byte, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		log.Printf("Failed to generate new JWT secret: %v", err)
		return nil, err
	}

	return secret, nil
}

func (s *AuthService) EncryptClientID(clientID string) (string, error) {
	s.mu.RLock()
	secret := s.jwtSecret
	s.mu.RUnlock()

	claims := jwt.MapClaims{
		"client_id": clientID,
		"exp":       time.Now().Add(s.config.JWTExpiresIn).Unix(),
		"iat":       time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func (s *AuthService) DecryptClientID(sessionID string) (string, error) {
	if sessionID == "" {
		return "", nil
	}

	s.mu.RLock()
	secret := s.jwtSecret
	s.mu.RUnlock()

	token, err := jwt.Parse(sessionID, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})

	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if clientID, ok := claims["client_id"].(string); ok {
			return clientID, nil
		}
	}

	return "", fmt.Errorf("invalid token")
}

func (s *AuthService) GetClientVisits(clientID string) []VisitRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if visits, exists := s.visits[clientID]; exists {
		visitsCopy := make([]VisitRecord, len(visits))
		copy(visitsCopy, visits)
		return visitsCopy
	}
	return nil
}

func (s *AuthService) UpdateClientVisits(clientID string, newVisits []VisitRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.visits[clientID] = newVisits
}

func (s *AuthService) authTOTP(totpCode string) bool {
	valid, err := totp.ValidateCustom(
		totpCode,
		s.config.TOTPSecret,
		time.Now().UTC(),
		totp.ValidateOpts{
			Period:    30,                // Standard TOTP period (seconds)
			Skew:      1,                 // Allow 1 period clock skew (e.g., 30 seconds before or after)
			Digits:    otp.DigitsSix,     // Standard 6 digits
			Algorithm: otp.AlgorithmSHA1, // Standard algorithm
		},
	)

	if err != nil {
		log.Println("authTOTP error: ", err)
		return false
	}

	return valid
}

func (s *AuthService) Authenticate(clientID, index string) (string, error) {
	now := time.Now()
	record := VisitRecord{
		Index: index,
		Time:  now.UnixMilli(),
	}

	clientVisits := s.GetClientVisits(clientID)

	if len(clientVisits) > 0 {
		intervalFromLast := record.Time - clientVisits[len(clientVisits)-1].Time
		if !(s.config.AuthClickIntervalMS-s.config.AuthClickOffsetMS < intervalFromLast &&
			intervalFromLast < s.config.AuthClickIntervalMS+s.config.AuthClickOffsetMS) {
			s.UpdateClientVisits(clientID, []VisitRecord{record})
			log.Println("intervalFromLast", intervalFromLast)
			return "", nil
		}

		intervalFromFirst := record.Time - clientVisits[0].Time
		if intervalFromFirst > s.config.AuthClickRangeMS+s.config.AuthClickOffsetMS {
			s.UpdateClientVisits(clientID, []VisitRecord{record})
			log.Println("intervalFromFirst", intervalFromFirst)
			return "", nil
		}
	}

	clientVisits = append(clientVisits, record)
	if len(clientVisits) > totpCodeLength {
		clientVisits = clientVisits[len(clientVisits)-totpCodeLength:]
	}

	s.UpdateClientVisits(clientID, clientVisits)

	codes := ""
	for _, v := range clientVisits {
		codes += v.Index
	}
	log.Println("indexes: ", codes)

	if len(codes) == totpCodeLength && s.authTOTP(codes) {
		log.Println("auth success!")

		sessionID, err := s.EncryptClientID(clientID)
		s.UpdateClientVisits(clientID, []VisitRecord{})
		s.authedClients = append(s.authedClients, clientID)

		return sessionID, err
	}

	return "", nil
}

func (s *AuthService) CheckAuth(sessionID, clientID string) bool {
	decryptedClientID, err := s.DecryptClientID(sessionID)
	if err != nil {
		return false
	}
	return decryptedClientID == clientID
}

func (s *AuthService) LogoutAll() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.authedClients) == 0 {
		log.Println("No authenticated clients to log out")
		return "not affected"
	}

	newSecret, err := genJWTSecret()
	if err != nil {
		log.Printf("Failed to generate new JWT secret: %v", err)
		return "failed to rotate secret"
	}

	s.jwtSecret = newSecret
	log.Println("JWT secret rotated - all existing sessions invalidated")

	return "logged out"
}

// GenerateTOTPSecret generates a new valid TOTP secret key.
// The generated secret will be a Base32 encoded string that can be used with Google Authenticator.
func GenerateTOTPSecret() (string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Koishi",
		AccountName: "Default Client",
		Algorithm:   otp.AlgorithmSHA1,
		Digits:      otp.DigitsSix,
		Period:      30,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate TOTP secret: %w", err)
	}
	return key.Secret(), nil
}
