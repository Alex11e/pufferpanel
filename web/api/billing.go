package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3/config"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"github.com/pufferpanel/pufferpanel/v3/response"
	"github.com/pufferpanel/pufferpanel/v3/scopes"
	"github.com/pufferpanel/pufferpanel/v3/services"
	"github.com/pufferpanel/pufferpanel/v3/utils"
	"github.com/pufferpanel/pufferpanel/v3/web/daemon"
	"gorm.io/gorm"
)

func registerBilling(g *gin.RouterGroup) {
	g.GET("/plans", listBillingPlans)
	g.GET("/nodes", listBillingNodes)
	g.GET("/options", billingOptions)
	g.GET("/me", listMyBillingPurchases)
	g.POST("/checkout", checkoutBillingPlan)
	g.GET("/purchases/:id", getBillingPurchase)
	g.GET("/purchases/:id/provision", getBillingProvisionPayload)
	g.POST("/webhook/stripe", stripeBillingWebhook)
	g.GET("/admin/plans", middleware.RequiresPermission(scopes.ScopeAdmin), listAllBillingPlans)
	g.POST("/admin/plans", middleware.RequiresPermission(scopes.ScopeAdmin), createBillingPlan)
	g.PUT("/admin/plans/:id", middleware.RequiresPermission(scopes.ScopeAdmin), updateBillingPlan)
	g.DELETE("/admin/plans/:id", middleware.RequiresPermission(scopes.ScopeAdmin), deleteBillingPlan)
	g.POST("/admin/purchases/grant", middleware.RequiresPermission(scopes.ScopeAdmin), grantBillingPlan)
	g.OPTIONS("/plans", response.CreateOptions("GET"))
	g.OPTIONS("/nodes", response.CreateOptions("GET"))
	g.OPTIONS("/options", response.CreateOptions("GET"))
	g.OPTIONS("/me", response.CreateOptions("GET"))
	g.OPTIONS("/checkout", response.CreateOptions("POST"))
	g.OPTIONS("/purchases/:id", response.CreateOptions("GET"))
	g.OPTIONS("/purchases/:id/provision", response.CreateOptions("GET"))
	g.OPTIONS("/webhook/stripe", response.CreateOptions("POST"))
	g.OPTIONS("/admin/plans", response.CreateOptions("GET", "POST"))
	g.OPTIONS("/admin/plans/:id", response.CreateOptions("PUT", "DELETE"))
	g.OPTIONS("/admin/purchases/grant", response.CreateOptions("POST"))
}

func billingOptions(c *gin.Context) {
	providers := make([]string, 0, 3)
	if strings.TrimSpace(os.Getenv("PUFFER_BILLING_STRIPE_SECRET_KEY")) != "" {
		providers = append(providers, "stripe")
	}
	if strings.TrimSpace(os.Getenv("PUFFER_BILLING_PAYPAL_CLIENT_ID")) != "" && strings.TrimSpace(os.Getenv("PUFFER_BILLING_PAYPAL_CLIENT_SECRET")) != "" {
		providers = append(providers, "paypal")
	}
	if strings.TrimSpace(os.Getenv("PUFFER_BILLING_BARION_POS_KEY")) != "" {
		providers = append(providers, "barion")
	}
	c.JSON(http.StatusOK, gin.H{"enabled": config.BillingEnabled.Value(), "providers": providers})
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
	AllowOneTime        bool   `json:"allowOneTime"`
	AllowMonthly        bool   `json:"allowMonthly"`
	AllowYearly         bool   `json:"allowYearly"`
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
		AllowOneTime: r.AllowOneTime, AllowMonthly: r.AllowMonthly, AllowYearly: r.AllowYearly,
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
		return plan.OneTimePriceMinor, plan.AllowOneTime
	case models.BillingCycleMonth:
		return plan.MonthlyPriceMinor, plan.AllowMonthly
	case models.BillingCycleYear:
		return plan.YearlyPriceMinor, plan.AllowYearly
	default:
		return 0, false
	}
}

var billingCheckoutMutex sync.Mutex

type billingCheckoutRequest struct {
	PlanID       uint                  `json:"planId"`
	BillingCycle string                `json:"billingCycle"`
	AutoRenew    bool                  `json:"autoRenew"`
	Provider     string                `json:"provider"`
	Server       models.ServerCreation `json:"server"`
}

func checkoutBillingPlan(c *gin.Context) {
	userValue, ok := c.Get("user")
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	user := userValue.(*models.User)
	var request billingCheckoutRequest
	if err := c.ShouldBindJSON(&request); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	db := middleware.GetDatabase(c)
	var plan models.BillingPlan
	if request.PlanID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "billing plan is required"}})
		return
	}
	if err := db.First(&plan, request.PlanID).Error; response.HandleError(c, err, http.StatusNotFound) {
		return
	}
	if !plan.Active {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"msg": "billing plan is inactive"}})
		return
	}
	price, cycleAllowed := billingCyclePrice(&plan, request.BillingCycle)
	if !cycleAllowed || price < 0 || (request.AutoRenew && request.BillingCycle == models.BillingCycleOnce) {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "billing cycle is not available for this plan"}})
		return
	}
	if request.Server.Environment.Type != "docker" || request.Server.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "a named Docker server definition is required"}})
		return
	}
	if request.Server.NodeId == 0 && !config.PanelEnabled.Value() {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "node is required"}})
		return
	}
	node, err := (&services.Node{DB: db}).Get(request.Server.NodeId)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	if plan.CPUCapacityMilli == 0 || plan.MemoryCapacityMB == 0 || plan.MemoryCapacityMB > (1<<40)/(1024*1024) || plan.CPUCapacityMilli > ^uint64(0)/1_000_000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "billing plan CPU and memory limits must be positive and valid"}})
		return
	}
	admin, err := (&services.Permission{DB: db}).HasPermission(user.ID, "", scopes.ScopeAdmin)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	if !admin && price > 0 && strings.ToLower(request.Provider) != "stripe" {
		c.JSON(http.StatusNotImplemented, gin.H{"error": gin.H{"msg": "the selected payment provider is not configured yet"}})
		return
	}
	request.Server.Users = []string{user.Username}
	hostConfig, _ := request.Server.Environment.Metadata["hostConfig"].(map[string]interface{})
	if hostConfig == nil {
		hostConfig = make(map[string]interface{})
	}
	memoryBytes := plan.MemoryCapacityMB * 1024 * 1024
	hostConfig["Memory"] = memoryBytes
	hostConfig["MemorySwap"] = memoryBytes
	hostConfig["NanoCpus"] = plan.CPUCapacityMilli * 1_000_000
	request.Server.Environment.Metadata["hostConfig"] = hostConfig
	payload, err := json.Marshal(request.Server)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	billingCheckoutMutex.Lock()
	capacity, usedCPU, usedMemory, capacityErr := billingNodeUsage(db, node)
	if capacityErr != nil {
		billingCheckoutMutex.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"msg": "node capacity could not be verified"}})
		return
	}
	var activePurchases int64
	countErr := db.Model(&models.BillingPurchase{}).
		Where("user_id = ? AND plan_id = ? AND status IN ? AND (expires_at IS NULL OR expires_at > ?)", user.ID, plan.ID, []string{models.BillingStatusPending, models.BillingStatusPaid}, time.Now()).
		Count(&activePurchases).Error
	if countErr != nil {
		billingCheckoutMutex.Unlock()
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"msg": "could not check existing plan purchases"}})
		return
	}
	if !admin && uint(activePurchases) >= plan.MaxServers {
		billingCheckoutMutex.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"msg": "the server limit for this plan has been reached"}})
		return
	}
	if !admin && !services.HasBillingCapacity(capacity, usedCPU, usedMemory, plan.CPUCapacityMilli, plan.MemoryCapacityMB) {
		billingCheckoutMutex.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"msg": "selected node is full for this package"}})
		return
	}
	status := models.BillingStatusPending
	provider := strings.ToLower(request.Provider)
	amount := price
	if admin || price == 0 {
		status, amount = models.BillingStatusPaid, 0
		if admin {
			provider = "admin"
		} else {
			provider = "free"
			request.AutoRenew = false
		}
	}
	now := time.Now()
	purchase := &models.BillingPurchase{
		UserID: user.ID, PlanID: plan.ID, NodeID: node.ID, Status: status,
		BillingCycle: request.BillingCycle, AutoRenew: request.AutoRenew,
		PaymentProvider: provider, Currency: plan.Currency, AmountMinor: amount,
		ReservedCPUCapacityMilli: plan.CPUCapacityMilli, ReservedMemoryCapacityMB: plan.MemoryCapacityMB,
		ProvisionPayload: payload,
	}
	if status == models.BillingStatusPending {
		holdUntil := now.Add(30 * time.Minute)
		purchase.ExpiresAt = &holdUntil
	} else {
		purchase.ExpiresAt = billingPlanExpiry(&plan, request.BillingCycle, now)
	}
	createErr := db.Create(purchase).Error
	billingCheckoutMutex.Unlock()
	if response.HandleError(c, createErr, http.StatusInternalServerError) {
		return
	}
	if status == models.BillingStatusPaid {
		c.JSON(http.StatusCreated, gin.H{"purchaseId": purchase.ID, "status": purchase.Status})
		return
	}
	session, err := createStripeCheckout(c.Request.Context(), purchase, &plan, config.MasterUrl.Value())
	if err != nil {
		purchase.Status = models.BillingStatusFailed
		_ = db.Save(purchase).Error
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"msg": err.Error()}})
		return
	}
	purchase.ProviderCheckoutID = session.ID
	if err = db.Save(purchase).Error; response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.JSON(http.StatusCreated, gin.H{"purchaseId": purchase.ID, "status": purchase.Status, "checkoutUrl": session.URL})
}

func billingNodeUsage(db *gorm.DB, node *models.Node) (services.BillingCapacity, uint64, uint64, error) {
	featuresResponse, err := (&services.Node{DB: db}).CallNode(node, http.MethodGet, "/daemon/features", nil, nil)
	if err != nil || featuresResponse == nil || featuresResponse.StatusCode != http.StatusOK {
		utils.CloseResponse(featuresResponse)
		return services.BillingCapacity{}, 0, 0, errors.New("node features are unavailable")
	}
	defer utils.CloseResponse(featuresResponse)
	var features daemon.Features
	if err = json.NewDecoder(featuresResponse.Body).Decode(&features); err != nil {
		return services.BillingCapacity{}, 0, 0, err
	}
	capacity := services.ResolveBillingCapacity(node, features.CPUCount, features.MemoryCapacityMB)
	var usage struct {
		CPU    uint64 `gorm:"column:cpu"`
		Memory uint64 `gorm:"column:memory"`
	}
	now := time.Now()
	err = db.Model(&models.BillingPurchase{}).
		Select("COALESCE(SUM(reserved_cpu_capacity_milli), 0) AS cpu, COALESCE(SUM(reserved_memory_capacity_mb), 0) AS memory").
		Where("node_id = ? AND status IN ? AND (expires_at IS NULL OR expires_at > ?)", node.ID, []string{models.BillingStatusPending, models.BillingStatusPaid}, now).
		Scan(&usage).Error
	return capacity, usage.CPU, usage.Memory, err
}

func getBillingPurchase(c *gin.Context) {
	userValue, ok := c.Get("user")
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "invalid purchase id"}})
		return
	}
	var purchase models.BillingPurchase
	if err = middleware.GetDatabase(c).Where("id = ? AND user_id = ?", id, userValue.(*models.User).ID).First(&purchase).Error; response.HandleError(c, err, http.StatusNotFound) {
		return
	}
	c.JSON(http.StatusOK, purchase)
}

func getBillingProvisionPayload(c *gin.Context) {
	userValue, ok := c.Get("user")
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "invalid purchase id"}})
		return
	}
	var purchase models.BillingPurchase
	if err = middleware.GetDatabase(c).Where("id = ? AND user_id = ?", id, userValue.(*models.User).ID).First(&purchase).Error; response.HandleError(c, err, http.StatusNotFound) {
		return
	}
	if purchase.Status != models.BillingStatusPaid || purchase.ServerIdentifier != nil {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"msg": "purchase is not ready for provisioning"}})
		return
	}
	var server models.ServerCreation
	if err = json.Unmarshal(purchase.ProvisionPayload, &server); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	server.BillingPurchaseID = purchase.ID
	c.JSON(http.StatusOK, server)
}

func validateBillingPurchaseForServer(db *gorm.DB, purchaseID, userID, nodeID uint, server *models.ServerCreation) (*models.BillingPurchase, error) {
	var purchase models.BillingPurchase
	if err := db.Where("id = ? AND user_id = ?", purchaseID, userID).First(&purchase).Error; err != nil {
		return nil, errors.New("billing purchase was not found for this user")
	}
	if purchase.Status != models.BillingStatusPaid || purchase.ServerIdentifier != nil {
		return nil, errors.New("billing purchase is not paid or has already been used")
	}
	if purchase.NodeID != nodeID {
		return nil, errors.New("billing purchase is reserved for a different node")
	}
	if purchase.ExpiresAt != nil && !purchase.ExpiresAt.After(time.Now()) {
		return nil, errors.New("billing purchase has expired")
	}
	if server.Environment.Type != "docker" {
		return nil, errors.New("billing resource limits require a Docker server")
	}
	if purchase.ReservedCPUCapacityMilli == 0 || purchase.ReservedMemoryCapacityMB == 0 {
		return nil, errors.New("billing purchase has invalid resource limits")
	}
	hostConfig, _ := server.Environment.Metadata["hostConfig"].(map[string]interface{})
	if hostConfig == nil {
		hostConfig = make(map[string]interface{})
	}
	memoryBytes := purchase.ReservedMemoryCapacityMB * 1024 * 1024
	hostConfig["Memory"] = memoryBytes
	hostConfig["MemorySwap"] = memoryBytes
	hostConfig["NanoCpus"] = purchase.ReservedCPUCapacityMilli * 1_000_000
	server.Environment.Metadata["hostConfig"] = hostConfig
	return &purchase, nil
}

func stripeBillingWebhook(c *gin.Context) {
	secret := strings.TrimSpace(os.Getenv("PUFFER_BILLING_STRIPE_WEBHOOK_SECRET"))
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil || !verifyStripeWebhook(body, c.GetHeader("Stripe-Signature"), secret, time.Now()) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	var event stripeWebhookEvent
	if err = json.Unmarshal(body, &event); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	var session stripeCheckoutObject
	if err = json.Unmarshal(event.Data.Object, &session); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	purchaseIDText := session.ClientReferenceID
	if purchaseIDText == "" && session.Metadata != nil {
		purchaseIDText = session.Metadata["purchase_id"]
	}
	purchaseID, err := strconv.ParseUint(purchaseIDText, 10, 32)
	if err != nil || purchaseID == 0 {
		c.Status(http.StatusOK)
		return
	}
	db := middleware.GetDatabase(c)
	var purchase models.BillingPurchase
	if err = db.First(&purchase, purchaseID).Error; response.HandleError(c, err, http.StatusNotFound) {
		return
	}
	if purchase.PaymentProvider != "stripe" || purchase.ProviderCheckoutID != session.ID {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	switch event.Type {
	case "checkout.session.completed", "checkout.session.async_payment_succeeded":
		if session.PaymentStatus == "paid" || (session.Mode == "subscription" && session.SubscriptionID != "") {
			var plan models.BillingPlan
			if err = db.First(&plan, purchase.PlanID).Error; response.HandleError(c, err, http.StatusInternalServerError) {
				return
			}
			purchase.Status = models.BillingStatusPaid
			purchase.ProviderPaymentID = session.PaymentIntent
			purchase.ProviderSubscriptionID = session.SubscriptionID
			purchase.ExpiresAt = billingPlanExpiry(&plan, purchase.BillingCycle, time.Now())
			if err = db.Save(&purchase).Error; response.HandleError(c, err, http.StatusInternalServerError) {
				return
			}
		}
	case "checkout.session.expired", "checkout.session.async_payment_failed":
		if purchase.Status == models.BillingStatusPending {
			purchase.Status = models.BillingStatusFailed
			_ = db.Save(&purchase).Error
		}
	}
	c.Status(http.StatusOK)
}
