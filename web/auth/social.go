package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	uuid "github.com/gofrs/uuid/v5"
	"github.com/pufferpanel/pufferpanel/v3/config"
	"github.com/pufferpanel/pufferpanel/v3/logging"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"github.com/pufferpanel/pufferpanel/v3/scopes"
	"github.com/pufferpanel/pufferpanel/v3/services"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type socialFlow struct {
	State            string
	Verifier         string
	Provider         string
	Mode             string
	UserID           uint
	SessionTokenHash string
	ExpiresAt        time.Time
}

type socialProviderPublicView struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Connectable bool   `json:"connectable"`
}

type socialIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
}

type socialOAuthProvider struct {
	config   oauth2.Config
	oidc     *oidc.Provider
	provider models.SocialProvider
}

func RegisterSocialRoutes(rg *gin.RouterGroup) {
	rg.GET("social/providers", middleware.NeedsDatabase, SocialProviders)
	rg.GET("social/callback", middleware.NeedsDatabase, SocialCallback)
	rg.GET("social/:provider", middleware.NeedsDatabase, SocialLoginStart)
}

func SocialProviders(c *gin.Context) {
	var providers []models.SocialProvider
	if err := middleware.GetDatabase(c).Where("enabled = ? AND client_id <> '' AND client_secret_encrypted <> ''", true).Order("name ASC").Find(&providers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "social login is unavailable"})
		return
	}
	result := make([]socialProviderPublicView, 0, len(providers))
	for _, provider := range providers {
		result = append(result, socialProviderPublicView{
			Key: provider.Key, Name: provider.Name, Kind: provider.Kind,
			Connectable: config.SocialLoginAllowAccountLinking.Value(),
		})
	}
	c.JSON(http.StatusOK, result)
}

func SocialLoginStart(c *gin.Context) {
	beginSocialFlow(c, "login", 0)
}

func BeginSocialConnect(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	if !config.SocialLoginAllowAccountLinking.Value() {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	beginSocialFlow(c, "connect", user.ID)
}

func beginSocialFlow(c *gin.Context, mode string, userID uint) {
	db := middleware.GetDatabase(c)
	var provider models.SocialProvider
	if err := db.Where("key = ? AND enabled = ?", c.Param("provider"), true).First(&provider).Error; err != nil {
		socialFlowFailure(c, mode, "provider_config", err)
		return
	}
	if mode == "connect" && userID == 0 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	providerConfig, err := makeOAuthProvider(ctx, provider)
	if err != nil {
		socialFlowFailure(c, mode, "provider_config", err)
		return
	}
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		socialFlowFailure(c, mode, "state", err)
		return
	}
	flow := socialFlow{
		State:     base64.RawURLEncoding.EncodeToString(stateBytes),
		Verifier:  oauth2.GenerateVerifier(),
		Provider:  provider.Key,
		Mode:      mode,
		UserID:    userID,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	if mode == "connect" {
		sessionToken, err := c.Cookie("puffer_auth")
		if err != nil || sessionToken == "" {
			socialFlowFailure(c, mode, "session", err)
			return
		}
		flow.SessionTokenHash, err = services.HashToken(sessionToken)
		if err != nil {
			socialFlowFailure(c, mode, "session", err)
			return
		}
	}
	if err := storeSocialFlow(db, flow); err != nil {
		socialFlowFailure(c, mode, "state_store", err)
		return
	}
	options := []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(flow.Verifier)}
	if provider.Kind == "google" || provider.Kind == "oidc" {
		options = append(options, oauth2.SetAuthURLParam("nonce", flow.State))
	}
	if flow.Mode == "login" {
		options = append(options, oauth2.SetAuthURLParam("prompt", "select_account"))
	}
	c.Redirect(http.StatusFound, providerConfig.config.AuthCodeURL(flow.State, options...))
}

func SocialCallback(c *gin.Context) {
	db := middleware.GetDatabase(c)
	flow, err := consumeSocialFlow(db, c.Query("state"))
	if err != nil {
		socialFlowFailure(c, "login", "state", err)
		return
	}
	if c.Query("error") != "" || c.Query("code") == "" {
		socialFlowFailure(c, flow.Mode, "provider_denied", nil)
		return
	}
	var provider models.SocialProvider
	if err := db.Where("key = ? AND enabled = ?", flow.Provider, true).First(&provider).Error; err != nil {
		socialFlowFailure(c, flow.Mode, "provider_config", err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	providerConfig, err := makeOAuthProvider(ctx, provider)
	if err != nil {
		socialFlowFailure(c, flow.Mode, "provider_config", err)
		return
	}
	token, err := providerConfig.config.Exchange(ctx, c.Query("code"), oauth2.VerifierOption(flow.Verifier))
	if err != nil {
		socialFlowFailure(c, flow.Mode, "exchange", err)
		return
	}
	identity, err := providerConfig.identity(ctx, token, flow.State)
	if err != nil || identity.Subject == "" {
		socialFlowFailure(c, flow.Mode, "identity", err)
		return
	}
	identity.Email = strings.ToLower(strings.TrimSpace(identity.Email))

	if flow.Mode == "connect" {
		if !config.SocialLoginAllowAccountLinking.Value() {
			socialFlowFailure(c, flow.Mode, "linking_disabled", nil)
			return
		}
		if !socialRequestUserMatches(db, flow) {
			socialFlowFailure(c, flow.Mode, "session", nil)
			return
		}
		var user models.User
		if err := db.First(&user, flow.UserID).Error; err != nil {
			socialFlowFailure(c, flow.Mode, "account_missing", err)
			return
		}
		if err := attachSocialIdentity(db, &user, provider, identity); err != nil {
			reason := "connection"
			if strings.Contains(err.Error(), "already connected") || strings.Contains(err.Error(), "different account") {
				reason = "account_conflict"
			}
			socialFlowFailure(c, flow.Mode, reason, err)
			return
		}
		issueSocialSessionTo(c, &user, flow.Mode, "/self?socialConnected=1#social")
		return
	}

	user, err := findOrCreateSocialUser(db, provider, identity)
	if err != nil {
		socialFlowFailure(c, flow.Mode, "account_policy", err)
		return
	}
	if user.OtpActive {
		loginSession := sessions.Default(c)
		loginSession.Clear()
		loginSession.Set("user", user.Email)
		loginSession.Set("time", time.Now().Unix())
		if err := loginSession.Save(); err != nil {
			socialFlowFailure(c, flow.Mode, "session", err)
			return
		}
		c.Redirect(http.StatusFound, "/auth/login?social2fa=1")
		return
	}
	issueSocialSession(c, user)
}

func makeOAuthProvider(ctx context.Context, provider models.SocialProvider) (*socialOAuthProvider, error) {
	secret, err := services.DecryptSocialSecret(provider.ClientSecretEncrypted)
	if err != nil {
		return nil, err
	}
	callbackURL := config.SocialLoginCallbackURL()
	if callbackURL == "" {
		return nil, errors.New("panel master URL is not configured")
	}
	result := &socialOAuthProvider{
		provider: provider,
		config: oauth2.Config{
			ClientID:     provider.ClientID,
			ClientSecret: secret,
			RedirectURL:  callbackURL,
		},
	}
	switch provider.Kind {
	case "google", "oidc":
		issuer := provider.IssuerURL
		if provider.Kind == "google" {
			issuer = "https://accounts.google.com"
		}
		if issuer == "" || !strings.HasPrefix(issuer, "https://") {
			return nil, errors.New("invalid OIDC issuer")
		}
		result.oidc, err = oidc.NewProvider(ctx, issuer)
		if err != nil {
			return nil, err
		}
		result.config.Endpoint = result.oidc.Endpoint()
		result.config.Scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	case "discord":
		result.config.Endpoint = oauth2.Endpoint{
			AuthURL:  "https://discord.com/oauth2/authorize",
			TokenURL: "https://discord.com/api/oauth2/token",
		}
		result.config.Scopes = []string{"identify", "email"}
	case "github":
		result.config.Endpoint = oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		}
		result.config.Scopes = []string{"read:user", "user:email"}
	default:
		return nil, fmt.Errorf("unsupported social provider kind %q", provider.Kind)
	}
	return result, nil
}

func (provider *socialOAuthProvider) identity(ctx context.Context, token *oauth2.Token, expectedNonce string) (socialIdentity, error) {
	if provider.oidc != nil {
		return provider.oidcIdentity(ctx, token, expectedNonce)
	}
	client := provider.config.Client(ctx, token)
	if provider.provider.Kind == "discord" {
		var profile struct {
			ID         string `json:"id"`
			Username   string `json:"username"`
			GlobalName string `json:"global_name"`
			Email      string `json:"email"`
			Verified   bool   `json:"verified"`
		}
		if err := getSocialJSON(ctx, client, "https://discord.com/api/users/@me", &profile); err != nil {
			return socialIdentity{}, err
		}
		name := profile.GlobalName
		if name == "" {
			name = profile.Username
		}
		return socialIdentity{Subject: profile.ID, Email: profile.Email, EmailVerified: profile.Verified, DisplayName: name}, nil
	}
	var profile struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return socialIdentity{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "PufferPanel")
	response, err := client.Do(request)
	if err != nil {
		return socialIdentity{}, err
	}
	if err = decodeSocialJSON(response, &profile); err != nil {
		return socialIdentity{}, err
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	request, err = http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return socialIdentity{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "PufferPanel")
	response, err = client.Do(request)
	if err != nil {
		return socialIdentity{}, err
	}
	if err = decodeSocialJSON(response, &emails); err != nil {
		return socialIdentity{}, err
	}
	identity := socialIdentity{Subject: strconv.FormatInt(profile.ID, 10), Email: profile.Email, DisplayName: profile.Name}
	for _, email := range emails {
		if email.Primary && email.Verified {
			identity.Email = email.Email
			identity.EmailVerified = true
			break
		}
	}
	if identity.DisplayName == "" {
		identity.DisplayName = profile.Login
	}
	return identity, nil
}

func (provider *socialOAuthProvider) oidcIdentity(ctx context.Context, token *oauth2.Token, expectedNonce string) (socialIdentity, error) {
	rawToken, ok := token.Extra("id_token").(string)
	if !ok || rawToken == "" {
		return socialIdentity{}, errors.New("OIDC provider did not return an ID token")
	}
	idToken, err := provider.oidc.Verifier(&oidc.Config{ClientID: provider.provider.ClientID}).Verify(ctx, rawToken)
	if err != nil {
		return socialIdentity{}, err
	}
	var claims struct {
		Subject       string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Nonce         string `json:"nonce"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return socialIdentity{}, err
	}
	if claims.Nonce != expectedNonce {
		return socialIdentity{}, errors.New("OIDC nonce mismatch")
	}
	return socialIdentity{Subject: claims.Subject, Email: claims.Email, EmailVerified: claims.EmailVerified, DisplayName: claims.Name}, nil
}

func getSocialJSON(ctx context.Context, client *http.Client, endpoint string, output interface{}) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	return decodeSocialJSON(response, output)
}

func decodeSocialJSON(response *http.Response, output interface{}) error {
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("identity provider returned HTTP %d", response.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(output)
}

func findOrCreateSocialUser(db *gorm.DB, provider models.SocialProvider, identity socialIdentity) (*models.User, error) {
	var connection models.SocialConnection
	err := db.Preload("User").Where("provider_id = ? AND subject = ?", provider.ID, identity.Subject).First(&connection).Error
	if err == nil {
		return &connection.User, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if !identity.EmailVerified || identity.Email == "" {
		return nil, errors.New("verified email is required")
	}
	var user models.User
	err = db.Where("email = ?", identity.Email).First(&user).Error
	if err == nil {
		if !config.SocialLoginAllowEmailLinking.Value() {
			return nil, errors.New("email account linking is disabled")
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		if !socialRegistrationAllowed() {
			return nil, errors.New("social registration is disabled")
		}
		user, err = createSocialUser(db, identity.Email)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	if err = attachSocialIdentity(db, &user, provider, identity); err != nil {
		return nil, err
	}
	return &user, nil
}

func socialRegistrationAllowed() bool {
	return config.RegistrationEnabled.Value() && config.SocialLoginAllowRegistration.Value()
}

func createSocialUser(db *gorm.DB, email string) (models.User, error) {
	user := models.User{Email: email}
	for attempt := 0; attempt < 5; attempt++ {
		id, err := uuid.NewV4()
		if err != nil {
			return user, err
		}
		user.Username = "social-" + strings.ReplaceAll(id.String()[:12], "-", "")
		var existing models.User
		err = db.Select("id").Where("username = ?", user.Username).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			break
		}
		if err != nil {
			return user, err
		}
	}
	passwordBytes := make([]byte, 32)
	if _, err := rand.Read(passwordBytes); err != nil {
		return user, err
	}
	if err := user.SetPassword(base64.RawURLEncoding.EncodeToString(passwordBytes)); err != nil {
		return user, err
	}
	if err := db.Create(&user).Error; err != nil {
		return user, err
	}
	permissions := &services.Permission{DB: db}
	perms, err := permissions.GetForUserAndServer(user.ID, "")
	if err != nil {
		return user, err
	}
	perms.Scopes = []*scopes.Scope{scopes.ScopeLogin}
	if err = permissions.UpdatePermissions(perms); err != nil {
		return user, err
	}
	return user, nil
}

func attachSocialIdentity(db *gorm.DB, user *models.User, provider models.SocialProvider, identity socialIdentity) error {
	var existing models.SocialConnection
	err := db.Where("provider_id = ? AND subject = ?", provider.ID, identity.Subject).First(&existing).Error
	if err == nil {
		if existing.UserID != user.ID {
			return errors.New("social identity is already connected")
		}
		existing.Email = identity.Email
		existing.DisplayName = identity.DisplayName
		return db.Save(&existing).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	err = db.Where("user_id = ? AND provider_id = ?", user.ID, provider.ID).First(&existing).Error
	if err == nil {
		if existing.Subject != identity.Subject {
			return errors.New("a different account is already connected for this provider")
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return db.Create(&models.SocialConnection{
		UserID: user.ID, ProviderID: provider.ID, Subject: identity.Subject,
		Email: identity.Email, DisplayName: identity.DisplayName,
	}).Error
}

func socialRequestUserMatches(db *gorm.DB, flow socialFlow) bool {
	if flow.UserID == 0 || flow.SessionTokenHash == "" {
		return false
	}
	var session models.Session
	err := db.Where("token = ? AND user_id = ? AND expiration_time > ?", flow.SessionTokenHash, flow.UserID, time.Now()).First(&session).Error
	return err == nil && session.UserId != nil && *session.UserId == flow.UserID
}

func issueSocialSession(c *gin.Context, user *models.User) {
	issueSocialSessionTo(c, user, "login", "/")
}

func issueSocialSessionTo(c *gin.Context, user *models.User, mode string, destination string) {
	db := middleware.GetDatabase(c)
	perms, err := (&services.Permission{DB: db}).GetForUserAndServer(user.ID, "")
	if err != nil || !scopes.ContainsScope(perms.Scopes, scopes.ScopeLogin) {
		socialFlowFailure(c, mode, "permissions", err)
		return
	}
	token, err := (&services.Session{DB: db}).CreateForUser(user)
	if err != nil {
		socialFlowFailure(c, mode, "session", err)
		return
	}
	scopeData, err := json.Marshal(perms.Scopes)
	if err != nil {
		socialFlowFailure(c, mode, "session", err)
		return
	}
	secure := config.PanelWebCookiesSecure.Value() || c.Request.TLS != nil
	maxAge := int(time.Hour / time.Second)
	path := config.PanelWebCookiesPath.Value()
	if path == "" {
		path = "/"
	}
	domain := config.PanelWebCookiesDomain.Value()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("puffer_auth", token, maxAge, path, domain, secure, true)
	c.SetCookie("puffer_auth_expires", "", maxAge, path, domain, secure, false)
	c.SetCookie("puffer_scopes", url.QueryEscape(string(scopeData)), maxAge, path, domain, secure, false)
	c.Redirect(http.StatusFound, destination)
}

func socialFlowFailure(c *gin.Context, mode string, reason string, cause error) {
	if cause != nil {
		logging.Error.Printf("Social %s flow failed at %s: %v", mode, reason, cause)
	} else {
		logging.Error.Printf("Social %s flow failed at %s", mode, reason)
	}
	if mode == "connect" {
		c.Redirect(http.StatusFound, "/self?socialError="+url.QueryEscape(reason)+"#social")
		return
	}
	c.Redirect(http.StatusFound, "/auth/login?socialError=1")
}

func storeSocialFlow(db *gorm.DB, flow socialFlow) error {
	stateHash, err := services.HashToken(flow.State)
	if err != nil {
		return err
	}
	verifierEncrypted, err := services.EncryptSocialSecret(flow.Verifier)
	if err != nil {
		return err
	}
	if err := db.Where("expires_at <= ?", time.Now()).Delete(&models.SocialLoginFlow{}).Error; err != nil {
		return err
	}
	return db.Create(&models.SocialLoginFlow{
		StateHash: stateHash, ProviderKey: flow.Provider, Mode: flow.Mode,
		UserID: flow.UserID, SessionTokenHash: flow.SessionTokenHash,
		VerifierEncrypted: verifierEncrypted, ExpiresAt: flow.ExpiresAt,
	}).Error
}

func consumeSocialFlow(db *gorm.DB, state string) (socialFlow, error) {
	if state == "" {
		return socialFlow{}, errors.New("missing OAuth state")
	}
	stateHash, err := services.HashToken(state)
	if err != nil {
		return socialFlow{}, err
	}
	var stored models.SocialLoginFlow
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("state_hash = ? AND expires_at > ?", stateHash, time.Now()).First(&stored).Error; err != nil {
			return err
		}
		result := tx.Delete(&stored)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		return socialFlow{}, err
	}
	verifier, err := services.DecryptSocialSecret(stored.VerifierEncrypted)
	if err != nil {
		return socialFlow{}, err
	}
	return socialFlow{
		State: state, Verifier: verifier, Provider: stored.ProviderKey,
		Mode: stored.Mode, UserID: stored.UserID,
		SessionTokenHash: stored.SessionTokenHash, ExpiresAt: stored.ExpiresAt,
	}, nil
}
