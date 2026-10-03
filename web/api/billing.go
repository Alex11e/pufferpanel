package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"github.com/pufferpanel/pufferpanel/v3/response"
	"github.com/pufferpanel/pufferpanel/v3/scopes"
	"github.com/pufferpanel/pufferpanel/v3/services"
	"github.com/pufferpanel/pufferpanel/v3/utils"
	"github.com/pufferpanel/pufferpanel/v3/web/daemon"
)

func registerBilling(g *gin.RouterGroup) {
	g.GET("/plans", listBillingPlans)
	g.GET("/nodes", listBillingNodes)
	g.GET("/me", listMyBillingPurchases)
	g.GET("/admin/plans", middleware.RequiresPermission(scopes.ScopeAdmin), listAllBillingPlans)
	g.POST("/admin/plans", middleware.RequiresPermission(scopes.ScopeAdmin), createBillingPlan)
	g.PUT("/admin/plans/:id", middleware.RequiresPermission(scopes.ScopeAdmin), updateBillingPlan)
	g.DELETE("/admin/plans/:id", middleware.RequiresPermission(scopes.ScopeAdmin), deleteBillingPlan)
	g.POST("/admin/purchases/grant", middleware.RequiresPermission(scopes.ScopeAdmin), grantBillingPlan)
	g.OPTIONS("/plans", response.CreateOptions("GET"))
	g.OPTIONS("/nodes", response.CreateOptions("GET"))
	g.OPTIONS("/me", response.CreateOptions("GET"))
	g.OPTIONS("/admin/plans", response.CreateOptions("GET", "POST"))
	g.OPTIONS("/admin/plans/:id", response.CreateOptions("PUT", "DELETE"))
	g.OPTIONS("/admin/purchases/grant", response.CreateOptions("POST"))
}

type billingNodeOption struct {
	ID                        uint   `json:"id"`
	Name                      string `json:"name"`
	Available                 bool   `json:"available"`
	AvailableCPUCapacityMilli uint64 `json:"availableCpuCapacityMilli"`
	AvailableMemoryCapacityMB uint64 `json:"availableMemoryCapacityMB"`
}

func listBillingNodes(c *gin.Context) {
	planID, err := strconv.ParseUint(c.Query("planId"), 10, 32)
	if err != nil || planID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "a valid planId is required"}})
		return
	}
	db := middleware.GetDatabase(c)
	var plan models.BillingPlan
	if err = db.First(&plan, uint(planID)).Error; response.HandleError(c, err, http.StatusNotFound) {
		return
	}
	if !plan.Active {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"msg": "billing plan is not active"}})
		return
	}
	nodes, err := (&services.Node{DB: db}).GetAll()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	options := make([]billingNodeOption, 0, len(nodes))
	for _, node := range nodes {
		option := billingNodeOption{ID: node.ID, Name: node.Name}
		featuresResponse, callErr := (&services.Node{DB: db}).CallNode(node, http.MethodGet, "/daemon/features", nil, nil)
		if callErr == nil && featuresResponse != nil && featuresResponse.StatusCode == http.StatusOK {
			var features daemon.Features
			decodeErr := json.NewDecoder(featuresResponse.Body).Decode(&features)
			utils.CloseResponse(featuresResponse)
			if decodeErr == nil {
				capacity := services.ResolveBillingCapacity(node, features.CPUCount, features.MemoryCapacityMB)
				var usage struct {
					CPU    uint64 `gorm:"column:cpu"`
					Memory uint64 `gorm:"column:memory"`
				}
				now := time.Now()
				usageErr := db.Model(&models.BillingPurchase{}).
					Select("COALESCE(SUM(reserved_cpu_capacity_milli), 0) AS cpu, COALESCE(SUM(reserved_memory_capacity_mb), 0) AS memory").
					Where("node_id = ? AND status IN ? AND (expires_at IS NULL OR expires_at > ?)", node.ID, []string{models.BillingStatusPending, models.BillingStatusPaid}, now).
					Scan(&usage).Error
				if usageErr == nil {
					if usage.CPU < capacity.CPUCapacityMilli {
						option.AvailableCPUCapacityMilli = capacity.CPUCapacityMilli - usage.CPU
					}
					if usage.Memory < capacity.MemoryCapacityMB {
						option.AvailableMemoryCapacityMB = capacity.MemoryCapacityMB - usage.Memory
					}
					option.Available = services.HasBillingCapacity(capacity, usage.CPU, usage.Memory, plan.CPUCapacityMilli, plan.MemoryCapacityMB)
				}
			}
		} else {
			utils.CloseResponse(featuresResponse)
		}
		options = append(options, option)
	}
	c.JSON(http.StatusOK, options)
}

func listBillingPlans(c *gin.Context) {
	var plans []models.BillingPlan
	if err := middleware.GetDatabase(c).Where("active = ?", true).Order("id").Find(&plans).Error; response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.JSON(http.StatusOK, plans)
}

func listAllBillingPlans(c *gin.Context) {
	var plans []models.BillingPlan
	if err := middleware.GetDatabase(c).Order("id").Find(&plans).Error; response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.JSON(http.StatusOK, plans)
}

func listMyBillingPurchases(c *gin.Context) {
	user, ok := c.Get("user")
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	var purchases []models.BillingPurchase
	if err := middleware.GetDatabase(c).Where("user_id = ? AND status = ? AND (expires_at IS NULL OR expires_at > ?)", user.(*models.User).ID, models.BillingStatusPaid, time.Now()).Order("created_at DESC").Find(&purchases).Error; response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.JSON(http.StatusOK, purchases)
}

type billingPlanRequest struct {
	Name                string `json:"name"`
	Description         string `json:"description"`
	Active              bool   `json:"active"`
	Currency            string `json:"currency"`
	OneTimePriceMinor   int64  `json:"oneTimePriceMinor"`
	MonthlyPriceMinor   int64  `json:"monthlyPriceMinor"`
	YearlyPriceMinor    int64  `json:"yearlyPriceMinor"`
	OneTimeDurationDays uint   `json:"oneTimeDurationDays"`
	CPUCapacityMilli    uint64 `json:"cpuCapacityMilli"`
	MemoryCapacityMB    uint64 `json:"memoryCapacityMB"`
	MaxServers          uint   `json:"maxServers"`
}

func (r billingPlanRequest) model() *models.BillingPlan {
	return &models.BillingPlan{
		Name: r.Name, Description: r.Description, Active: r.Active, Currency: r.Currency,
		OneTimePriceMinor: r.OneTimePriceMinor, MonthlyPriceMinor: r.MonthlyPriceMinor,
		YearlyPriceMinor: r.YearlyPriceMinor, OneTimeDurationDays: r.OneTimeDurationDays,
		CPUCapacityMilli: r.CPUCapacityMilli, MemoryCapacityMB: r.MemoryCapacityMB, MaxServers: r.MaxServers,
	}
}

func parseBillingPlanID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "invalid plan id"}})
		return 0, false
	}
	return uint(id), true
}

func createBillingPlan(c *gin.Context) {
	var request billingPlanRequest
	if err := c.ShouldBindJSON(&request); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	plan := request.model()
	if err := middleware.GetDatabase(c).Create(plan).Error; response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	c.JSON(http.StatusCreated, plan)
}

func updateBillingPlan(c *gin.Context) {
	id, ok := parseBillingPlanID(c)
	if !ok {
		return
	}
	var request billingPlanRequest
	if err := c.ShouldBindJSON(&request); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	db := middleware.GetDatabase(c)
	var plan models.BillingPlan
	if err := db.First(&plan, id).Error; response.HandleError(c, err, http.StatusNotFound) {
		return
	}
	updated := request.model()
	updated.ID = plan.ID
	updated.CreatedAt = plan.CreatedAt
	if err := db.Save(updated).Error; response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	c.JSON(http.StatusOK, updated)
}

func deleteBillingPlan(c *gin.Context) {
	id, ok := parseBillingPlanID(c)
	if !ok {
		return
	}
	db := middleware.GetDatabase(c)
	var plan models.BillingPlan
	if err := db.First(&plan, id).Error; response.HandleError(c, err, http.StatusNotFound) {
		return
	}
	var purchaseCount int64
	if err := db.Model(&models.BillingPurchase{}).Where("plan_id = ?", id).Count(&purchaseCount).Error; response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	if purchaseCount > 0 {
		plan.Active = false
		if err := db.Save(&plan).Error; response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"active": false, "archived": true})
		return
	}
	if err := db.Delete(&plan).Error; response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)
}

type billingGrantRequest struct {
	UserID       uint   `json:"userId"`
	PlanID       uint   `json:"planId"`
	NodeID       uint   `json:"nodeId"`
	BillingCycle string `json:"billingCycle"`
}

func grantBillingPlan(c *gin.Context) {
	var request billingGrantRequest
	if err := c.ShouldBindJSON(&request); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	if request.UserID == 0 || request.PlanID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "user and plan are required"}})
		return
	}
	if request.BillingCycle != models.BillingCycleOnce && request.BillingCycle != models.BillingCycleMonth && request.BillingCycle != models.BillingCycleYear {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "invalid billing cycle"}})
		return
	}
	db := middleware.GetDatabase(c)
	var user models.User
	if err := db.First(&user, request.UserID).Error; response.HandleError(c, err, http.StatusNotFound) {
		return
	}
	var plan models.BillingPlan
	if err := db.First(&plan, request.PlanID).Error; response.HandleError(c, err, http.StatusNotFound) {
		return
	}
	if !plan.Active {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"msg": "inactive plans cannot be granted"}})
		return
	}
	if _, err := (&services.Node{DB: db}).Get(request.NodeID); response.HandleError(c, err, http.StatusNotFound) {
		return
	}
	expiresAt := billingPlanExpiry(&plan, request.BillingCycle, time.Now())
	purchase := models.BillingPurchase{
		UserID: request.UserID, PlanID: plan.ID, NodeID: request.NodeID,
		Status: models.BillingStatusPaid, BillingCycle: request.BillingCycle,
		PaymentProvider: "admin", Currency: plan.Currency, AmountMinor: 0,
		ReservedCPUCapacityMilli: plan.CPUCapacityMilli, ReservedMemoryCapacityMB: plan.MemoryCapacityMB,
		ExpiresAt: expiresAt,
	}
	if err := db.Create(&purchase).Error; response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.JSON(http.StatusCreated, purchase)
}

func billingPlanExpiry(plan *models.BillingPlan, cycle string, now time.Time) *time.Time {
	var expiresAt time.Time
	switch cycle {
	case models.BillingCycleOnce:
		if plan.OneTimeDurationDays == 0 {
			return nil
		}
		expiresAt = now.AddDate(0, 0, int(plan.OneTimeDurationDays))
	case models.BillingCycleMonth:
		expiresAt = now.AddDate(0, 1, 0)
	case models.BillingCycleYear:
		expiresAt = now.AddDate(1, 0, 0)
	}
	return &expiresAt
}

func billingCyclePrice(plan *models.BillingPlan, cycle string) (int64, bool) {
	switch strings.ToLower(strings.TrimSpace(cycle)) {
	case models.BillingCycleOnce:
		return plan.OneTimePriceMinor, true
	case models.BillingCycleMonth:
		return plan.MonthlyPriceMinor, true
	case models.BillingCycleYear:
		return plan.YearlyPriceMinor, true
	default:
		return 0, false
	}
}
