package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

type appConfig struct {
	clientID      string
	clientSecret  string
	userPoolID    string
	region        string
	returnJSONObj bool // when true, body is a native map (for local testing)
}

func loadConfig() (appConfig, error) {
	clientID := os.Getenv("COGNITO_CLIENT_ID")
	if clientID == "" {
		return appConfig{}, errors.New("COGNITO_CLIENT_ID not configured")
	}

	returnJSONObj := false
	if v := strings.ToLower(os.Getenv("RETURN_JSON_OBJECT")); v == "1" || v == "true" || v == "yes" {
		returnJSONObj = true
	}

	return appConfig{
		clientID:      clientID,
		clientSecret:  os.Getenv("COGNITO_CLIENT_SECRET"),
		userPoolID:    os.Getenv("COGNITO_USER_POOL_ID"),
		region:        os.Getenv("AWS_REGION"),
		returnJSONObj: returnJSONObj,
	}, nil
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token     *string `json:"token"`
	ExpiresIn *int32  `json:"expires_in"`
	TokenType *string `json:"token_type"`
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type challengeResponse struct {
	Error     string  `json:"error"`
	Message   string  `json:"message"`
	Challenge *string `json:"challenge"`
}

type handler struct {
	cfg     appConfig
	logger  *slog.Logger
	cognito *cognitoidentityprovider.Client
}

func newHandler(ctx context.Context, cfg appConfig) (*handler, error) {
	opts := []func(*config.LoadOptions) error{}
	if cfg.region != "" {
		opts = append(opts, config.WithRegion(cfg.region))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	logLevel := slog.LevelInfo
	if strings.ToUpper(os.Getenv("LOG_LEVEL")) == "DEBUG" {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))

	return &handler{
		cfg:     cfg,
		logger:  logger,
		cognito: cognitoidentityprovider.NewFromConfig(awsCfg),
	}, nil
}

func (h *handler) Handle(ctx context.Context, event events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	h.logger.InfoContext(ctx, "handle_auth invoked")

	rawBody := event.Body
	if rawBody == "" {
		return h.errorResponse(http.StatusBadRequest, "invalid_request", "Request body is required."), nil
	}

	if event.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(rawBody)
		if err != nil {
			return h.errorResponse(http.StatusBadRequest, "invalid_request", "Invalid base64 body"), nil
		}
		rawBody = string(decoded)
	}

	var req authRequest
	if err := json.Unmarshal([]byte(rawBody), &req); err != nil {
		return h.errorResponse(http.StatusBadRequest, "invalid_request", "Request body must be valid JSON"), nil
	}

	if req.Email == "" || req.Password == "" {
		return h.errorResponse(http.StatusBadRequest, "invalid_request", "Both email and password are required."), nil
	}

	// Using email as the username for Cognito.
	authParams := map[string]string{
		"USERNAME": req.Email,
		"PASSWORD": req.Password,
	}

	if h.cfg.clientSecret != "" {
		secretHash, err := calcSecretHash(req.Email, h.cfg.clientID, h.cfg.clientSecret)
		if err != nil {
			h.logger.ErrorContext(ctx, "failed to calculate secret hash", "error", err)
			return h.errorResponse(http.StatusInternalServerError, "server_error", "Failed to calculate client secret hash."), nil
		}
		authParams["SECRET_HASH"] = secretHash
	}

	input := &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow:       types.AuthFlowTypeUserPasswordAuth,
		AuthParameters: authParams,
		ClientId:       aws.String(h.cfg.clientID),
	}

	resp, err := h.cognito.InitiateAuth(ctx, input)
	if err != nil {
		return h.handleCognitoError(ctx, err), nil
	}

	if resp.AuthenticationResult == nil {
		challenge := string(resp.ChallengeName)
		h.logger.InfoContext(ctx, "cognito returned a challenge", "challenge", challenge)
		body, _ := json.Marshal(challengeResponse{
			Error:     "challenge_required",
			Message:   "Additional challenge required",
			Challenge: &challenge,
		})
		return h.rawResponse(http.StatusForbidden, string(body)), nil
	}

	result := resp.AuthenticationResult
	token := result.IdToken
	if token == nil {
		token = result.AccessToken
	}

	h.logger.InfoContext(ctx, "authentication successful", "email", req.Email)

	body, _ := json.Marshal(authResponse{
		Token:     token,
		ExpiresIn: &result.ExpiresIn,
		TokenType: result.TokenType,
	})
	return h.rawResponse(http.StatusOK, string(body)), nil
}

func (h *handler) handleCognitoError(ctx context.Context, err error) events.APIGatewayV2HTTPResponse {
	var notAuth *types.NotAuthorizedException
	if errors.As(err, &notAuth) {
		return h.errorResponse(http.StatusUnauthorized, "invalid_credentials", "Invalid email or password.")
	}

	var userNotFound *types.UserNotFoundException
	if errors.As(err, &userNotFound) {
		return h.errorResponse(http.StatusNotFound, "user_not_found", "User does not exist.")
	}

	var notConfirmed *types.UserNotConfirmedException
	if errors.As(err, &notConfirmed) {
		return h.errorResponse(http.StatusForbidden, "user_not_confirmed", "User not confirmed.")
	}

	var passReset *types.PasswordResetRequiredException
	if errors.As(err, &passReset) {
		return h.errorResponse(http.StatusForbidden, "password_reset_required", "Password reset required.")
	}

	h.logger.ErrorContext(ctx, "unhandled cognito error", "error", err)
	return h.errorResponse(http.StatusBadGateway, "upstream_error", "Cognito error: "+err.Error())
}

var defaultHeaders = map[string]string{
	"Content-Type":                "application/json",
	"Access-Control-Allow-Origin": "*",
}

func (h *handler) errorResponse(statusCode int, errCode, message string) events.APIGatewayV2HTTPResponse {
	body, _ := json.Marshal(errorResponse{Error: errCode, Message: message})
	return h.rawResponse(statusCode, string(body))
}

func (h *handler) rawResponse(statusCode int, body string) events.APIGatewayV2HTTPResponse {
	return events.APIGatewayV2HTTPResponse{
		StatusCode:      statusCode,
		Headers:         defaultHeaders,
		Body:            body,
		IsBase64Encoded: false,
	}
}

// calcSecretHash computes: Base64( HMAC-SHA256( clientSecret, username + clientID ) )
func calcSecretHash(username, clientID, clientSecret string) (string, error) {
	mac := hmac.New(sha256.New, []byte(clientSecret))
	if _, err := mac.Write([]byte(username + clientID)); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

func main() {
	ctx := context.Background()

	cfg, err := loadConfig()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	h, err := newHandler(ctx, cfg)
	if err != nil {
		slog.Error("failed to initialize handler", "error", err)
		os.Exit(1)
	}

	lambda.Start(h.Handle)
}
