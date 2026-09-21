package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"github.com/pufferpanel/pufferpanel/v3/response"
	"github.com/pufferpanel/pufferpanel/v3/scopes"
	"github.com/pufferpanel/pufferpanel/v3/services"
)

type ticketView struct {
	ID        uint                          `json:"id"`
	UserID    uint                          `json:"userId"`
	Username  string                        `json:"username"`
	ServerID  string                        `json:"serverId,omitempty"`
	Subject   string                        `json:"subject"`
	Status    string                        `json:"status"`
	CreatedAt string                        `json:"createdAt"`
	UpdatedAt string                        `json:"updatedAt"`
	Messages  []models.SupportTicketMessage `json:"messages,omitempty"`
}

func registerTickets(g *gin.RouterGroup) {
	g.Use(middleware.RequiresPermission(scopes.ScopeLogin))
	g.GET("", listTickets)
	g.POST("", createTicket)
	g.GET("/:id", getTicket)
	g.POST("/:id/messages", addTicketMessage)
	g.PUT("/:id/status", updateTicketStatus)
	g.OPTIONS("", response.CreateOptions("GET", "POST"))
	g.OPTIONS("/:id", response.CreateOptions("GET"))
	g.OPTIONS("/:id/messages", response.CreateOptions("POST"))
	g.OPTIONS("/:id/status", response.CreateOptions("PUT"))
}

func ticketIsAdmin(c *gin.Context, user *models.User) bool {
	permission, err := (&services.Permission{DB: middleware.GetDatabase(c)}).GetForUserAndServer(user.ID, "")
	return err == nil && permission != nil && scopes.ContainsScope(permission.Scopes, scopes.ScopeAdmin)
}

func ticketModelView(c *gin.Context, ticket *models.SupportTicket, includeMessages bool) ticketView {
	view := ticketView{ID: ticket.ID, UserID: ticket.UserID, ServerID: ticket.ServerIdentifier, Subject: ticket.Subject, Status: ticket.Status, CreatedAt: ticket.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: ticket.UpdatedAt.UTC().Format(time.RFC3339)}
	var user models.User
	if middleware.GetDatabase(c).Select("username").First(&user, ticket.UserID).Error == nil {
		view.Username = user.Username
	}
	if includeMessages {
		_ = middleware.GetDatabase(c).Where("ticket_id = ?", ticket.ID).Order("created_at ASC").Find(&view.Messages).Error
	}
	return view
}

func ticketAllowed(c *gin.Context, ticket *models.SupportTicket) (user *models.User, admin bool, allowed bool) {
	user = c.MustGet("user").(*models.User)
	admin = ticketIsAdmin(c, user)
	return user, admin, admin || ticket.UserID == user.ID
}

func listTickets(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	query := middleware.GetDatabase(c).Order("updated_at DESC").Limit(100)
	if !ticketIsAdmin(c, user) {
		query = query.Where("user_id = ?", user.ID)
	}
	var tickets []models.SupportTicket
	if err := query.Find(&tickets).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	result := make([]ticketView, 0, len(tickets))
	for i := range tickets {
		result = append(result, ticketModelView(c, &tickets[i], false))
	}
	c.JSON(http.StatusOK, result)
}

type createTicketRequest struct {
	Subject  string `json:"subject"`
	Message  string `json:"message"`
	ServerID string `json:"serverId"`
}

func createTicket(c *gin.Context) {
	var request createTicketRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket"})
		return
	}
	request.Subject, request.Message, request.ServerID = strings.TrimSpace(request.Subject), strings.TrimSpace(request.Message), strings.TrimSpace(request.ServerID)
	if request.Subject == "" || len(request.Subject) > 140 || request.Message == "" || len(request.Message) > 4000 || len(request.ServerID) > 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket"})
		return
	}
	user := c.MustGet("user").(*models.User)
	ticket := models.SupportTicket{UserID: user.ID, ServerIdentifier: request.ServerID, Subject: request.Subject, Status: "open"}
	db := middleware.GetDatabase(c)
	if err := db.Create(&ticket).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	message := models.SupportTicketMessage{TicketID: ticket.ID, UserID: user.ID, Username: user.Username, Body: request.Message}
	if err := db.Create(&message).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusCreated, ticketModelView(c, &ticket, true))
}

func loadTicket(c *gin.Context) *models.SupportTicket {
	var ticket models.SupportTicket
	if err := middleware.GetDatabase(c).First(&ticket, c.Param("id")).Error; err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return nil
	}
	return &ticket
}

func getTicket(c *gin.Context) {
	ticket := loadTicket(c)
	if ticket == nil {
		return
	}
	_, _, allowed := ticketAllowed(c, ticket)
	if !allowed {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	c.JSON(http.StatusOK, ticketModelView(c, ticket, true))
}

type ticketMessageRequest struct {
	Body string `json:"body"`
}

func addTicketMessage(c *gin.Context) {
	ticket := loadTicket(c)
	if ticket == nil {
		return
	}
	user, admin, allowed := ticketAllowed(c, ticket)
	if !allowed {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	var request ticketMessageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message"})
		return
	}
	request.Body = strings.TrimSpace(request.Body)
	if request.Body == "" || len(request.Body) > 4000 || ticket.Status == "closed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ticket is closed or message is invalid"})
		return
	}
	message := models.SupportTicketMessage{TicketID: ticket.ID, UserID: user.ID, Username: user.Username, Body: request.Body}
	if err := middleware.GetDatabase(c).Create(&message).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if admin {
		ticket.Status = "answered"
	} else {
		ticket.Status = "open"
	}
	_ = middleware.GetDatabase(c).Save(ticket).Error
	c.JSON(http.StatusCreated, message)
}

type ticketStatusRequest struct {
	Status string `json:"status"`
}

func updateTicketStatus(c *gin.Context) {
	ticket := loadTicket(c)
	if ticket == nil {
		return
	}
	_, admin, allowed := ticketAllowed(c, ticket)
	if !allowed {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	var request ticketStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}
	request.Status = strings.ToLower(strings.TrimSpace(request.Status))
	if request.Status != "closed" && !(admin && (request.Status == "open" || request.Status == "answered")) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	ticket.Status = request.Status
	if err := middleware.GetDatabase(c).Save(ticket).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, ticketModelView(c, ticket, false))
}
