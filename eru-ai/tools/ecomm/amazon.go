package ecomm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	utils "github.com/eru-tech/eru/eru-utils"
)

type AmazonTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// Amazon SP-API FinancialEvents Response Structures
type FinancialEventsResponse struct {
	Payload struct {
		FinancialEvents FinancialEvents `json:"FinancialEvents"`
		NextToken       string          `json:"NextToken,omitempty"`
	} `json:"payload"`
}

type FinancialEvents struct {
	ShipmentEventList                      []ShipmentEvent                      `json:"ShipmentEventList,omitempty"`
	RefundEventList                        []ShipmentEvent                      `json:"RefundEventList,omitempty"`
	GuaranteeClaimEventList                []ShipmentEvent                      `json:"GuaranteeClaimEventList,omitempty"`
	ChargebackEventList                    []ShipmentEvent                      `json:"ChargebackEventList,omitempty"`
	PayWithAmazonEventList                 []PayWithAmazonEvent                 `json:"PayWithAmazonEventList,omitempty"`
	ServiceProviderCreditEventList         []SolutionProviderCreditEvent        `json:"ServiceProviderCreditEventList,omitempty"`
	RetrochargeEventList                   []RetrochargeEvent                   `json:"RetrochargeEventList,omitempty"`
	RentalTransactionEventList             []RentalTransactionEvent             `json:"RentalTransactionEventList,omitempty"`
	ProductAdsPaymentEventList             []ProductAdsPaymentEvent             `json:"ProductAdsPaymentEventList,omitempty"`
	ServiceFeeEventList                    []ServiceFeeEvent                    `json:"ServiceFeeEventList,omitempty"`
	SellerDealPaymentEventList             []SellerDealPaymentEvent             `json:"SellerDealPaymentEventList,omitempty"`
	DebtRecoveryEventList                  []DebtRecoveryEvent                  `json:"DebtRecoveryEventList,omitempty"`
	LoanServicingEventList                 []LoanServicingEvent                 `json:"LoanServicingEventList,omitempty"`
	AdjustmentEventList                    []AdjustmentEvent                    `json:"AdjustmentEventList,omitempty"`
	SAFETReimbursementEventList            []SAFETReimbursementEvent            `json:"SAFETReimbursementEventList,omitempty"`
	SellerReviewEnrollmentPaymentEventList []SellerReviewEnrollmentPaymentEvent `json:"SellerReviewEnrollmentPaymentEventList,omitempty"`
	FBALiquidationEventList                []FBALiquidationEvent                `json:"FBALiquidationEventList,omitempty"`
	CouponPaymentEventList                 []CouponPaymentEvent                 `json:"CouponPaymentEventList,omitempty"`
	ImagingServicesFeeEventList            []ImagingServicesFeeEvent            `json:"ImagingServicesFeeEventList,omitempty"`
	NetworkComminglingTransactionEventList []NetworkComminglingTransactionEvent `json:"NetworkComminglingTransactionEventList,omitempty"`
	AffordabilityExpenseEventList          []AffordabilityExpenseEvent          `json:"AffordabilityExpenseEventList,omitempty"`
	AffordabilityExpenseReversalEventList  []AffordabilityExpenseEvent          `json:"AffordabilityExpenseReversalEventList,omitempty"`
	RemovalShipmentEventList               []RemovalShipmentEvent               `json:"RemovalShipmentEventList,omitempty"`
	RemovalShipmentAdjustmentEventList     []RemovalShipmentAdjustmentEvent     `json:"RemovalShipmentAdjustmentEventList,omitempty"`
	TrialShipmentEventList                 []TrialShipmentEvent                 `json:"TrialShipmentEventList,omitempty"`
	TDSReimbursementEventList              []TDSReimbursementEvent              `json:"TDSReimbursementEventList,omitempty"`
	AdhocDisbursementEventList             []AdhocDisbursementEvent             `json:"AdhocDisbursementEventList,omitempty"`
	TaxWithholdingEventList                []TaxWithholdingEvent                `json:"TaxWithholdingEventList,omitempty"`
	ChargeRefundEventList                  []ChargeRefundEvent                  `json:"ChargeRefundEventList,omitempty"`
	FailedAdhocDisbursementEventList       []FailedAdhocDisbursementEvent       `json:"FailedAdhocDisbursementEventList,omitempty"`
	ValueAddedServiceChargeEventList       []ValueAddedServiceChargeEvent       `json:"ValueAddedServiceChargeEventList,omitempty"`
	CapacityReservationBillingEventList    []CapacityReservationBillingEvent    `json:"CapacityReservationBillingEventList,omitempty"`
	PaymentEventList                       []DebtRecoveryEvent                  `json:"PaymentEventList,omitempty"`
	CreateInboundShipmentPlanEventList     []CreateInboundShipmentPlanEvent     `json:"CreateInboundShipmentPlanEventList,omitempty"`
	ShippingLabelEventList                 []ShippingLabelEvent                 `json:"ShippingLabelEventList,omitempty"`
}

type Currency struct {
	CurrencyCode   string  `json:"CurrencyCode"`
	CurrencyAmount float64 `json:"CurrencyAmount"`
}

type ShipmentEvent struct {
	AmazonOrderId              string            `json:"AmazonOrderId,omitempty"`
	SellerOrderId              string            `json:"SellerOrderId,omitempty"`
	MarketplaceName            string            `json:"MarketplaceName,omitempty"`
	OrderChargeList            []ChargeComponent `json:"OrderChargeList,omitempty"`
	OrderChargeAdjustmentList  []ChargeComponent `json:"OrderChargeAdjustmentList,omitempty"`
	ShipmentFeeList            []FeeComponent    `json:"ShipmentFeeList,omitempty"`
	ShipmentFeeAdjustmentList  []FeeComponent    `json:"ShipmentFeeAdjustmentList,omitempty"`
	OrderFeeList               []FeeComponent    `json:"OrderFeeList,omitempty"`
	OrderFeeAdjustmentList     []FeeComponent    `json:"OrderFeeAdjustmentList,omitempty"`
	DirectPaymentList          []DirectPayment   `json:"DirectPaymentList,omitempty"`
	PostedDate                 string            `json:"PostedDate,omitempty"`
	ShipmentItemList           []ShipmentItem    `json:"ShipmentItemList,omitempty"`
	ShipmentItemAdjustmentList []ShipmentItem    `json:"ShipmentItemAdjustmentList,omitempty"`
}

type ChargeComponent struct {
	ChargeType   string   `json:"ChargeType,omitempty"`
	ChargeAmount Currency `json:"ChargeAmount,omitempty"`
}

type FeeComponent struct {
	FeeType   string   `json:"FeeType,omitempty"`
	FeeAmount Currency `json:"FeeAmount,omitempty"`
}

type DirectPayment struct {
	DirectPaymentType   string   `json:"DirectPaymentType,omitempty"`
	DirectPaymentAmount Currency `json:"DirectPaymentAmount,omitempty"`
}

type ShipmentItem struct {
	SellerSKU                string                 `json:"SellerSKU,omitempty"`
	OrderItemId              string                 `json:"OrderItemId,omitempty"`
	OrderAdjustmentItemId    string                 `json:"OrderAdjustmentItemId,omitempty"`
	QuantityShipped          int                    `json:"QuantityShipped,omitempty"`
	ItemChargeList           []ChargeComponent      `json:"ItemChargeList,omitempty"`
	ItemChargeAdjustmentList []ChargeComponent      `json:"ItemChargeAdjustmentList,omitempty"`
	ItemFeeList              []FeeComponent         `json:"ItemFeeList,omitempty"`
	ItemFeeAdjustmentList    []FeeComponent         `json:"ItemFeeAdjustmentList,omitempty"`
	ItemTaxWithheldList      []TaxWithheldComponent `json:"ItemTaxWithheldList,omitempty"`
	PromotionList            []Promotion            `json:"PromotionList,omitempty"`
	PromotionAdjustmentList  []Promotion            `json:"PromotionAdjustmentList,omitempty"`
	CostOfPointsGranted      Currency               `json:"CostOfPointsGranted,omitempty"`
	CostOfPointsReturned     Currency               `json:"CostOfPointsReturned,omitempty"`
}

type TaxWithheldComponent struct {
	TaxCollectionModel string     `json:"TaxCollectionModel,omitempty"`
	TaxesWithheld      []Currency `json:"TaxesWithheld,omitempty"`
}

type Promotion struct {
	PromotionType   string   `json:"PromotionType,omitempty"`
	PromotionId     string   `json:"PromotionId,omitempty"`
	PromotionAmount Currency `json:"PromotionAmount,omitempty"`
}

type PayWithAmazonEvent struct {
	SellerOrderId         string          `json:"SellerOrderId,omitempty"`
	TransactionPostedDate string          `json:"TransactionPostedDate,omitempty"`
	BusinessObjectType    string          `json:"BusinessObjectType,omitempty"`
	SalesChannel          string          `json:"SalesChannel,omitempty"`
	Charge                ChargeComponent `json:"Charge,omitempty"`
	FeeList               []FeeComponent  `json:"FeeList,omitempty"`
	PaymentAmountType     string          `json:"PaymentAmountType,omitempty"`
	AmountDescription     string          `json:"AmountDescription,omitempty"`
	FulfillmentChannel    string          `json:"FulfillmentChannel,omitempty"`
	StoreName             string          `json:"StoreName,omitempty"`
}

type SolutionProviderCreditEvent struct {
	ProviderTransactionType string   `json:"ProviderTransactionType,omitempty"`
	SellerOrderId           string   `json:"SellerOrderId,omitempty"`
	MarketplaceId           string   `json:"MarketplaceId,omitempty"`
	MarketplaceCountryCode  string   `json:"MarketplaceCountryCode,omitempty"`
	SellerId                string   `json:"SellerId,omitempty"`
	SellerStoreName         string   `json:"SellerStoreName,omitempty"`
	ProviderId              string   `json:"ProviderId,omitempty"`
	ProviderStoreName       string   `json:"ProviderStoreName,omitempty"`
	TransactionAmount       Currency `json:"TransactionAmount,omitempty"`
	TransactionCreationDate string   `json:"TransactionCreationDate,omitempty"`
}

type RetrochargeEvent struct {
	RetrochargeEventType       string                 `json:"RetrochargeEventType,omitempty"`
	AmazonOrderId              string                 `json:"AmazonOrderId,omitempty"`
	PostedDate                 string                 `json:"PostedDate,omitempty"`
	BaseTax                    Currency               `json:"BaseTax,omitempty"`
	ShippingTax                Currency               `json:"ShippingTax,omitempty"`
	MarketplaceName            string                 `json:"MarketplaceName,omitempty"`
	RetrochargeTaxWithheldList []TaxWithheldComponent `json:"RetrochargeTaxWithheldList,omitempty"`
}

type RentalTransactionEvent struct {
	AmazonOrderId         string                 `json:"AmazonOrderId,omitempty"`
	RentalEventType       string                 `json:"RentalEventType,omitempty"`
	ExtensionLength       int                    `json:"ExtensionLength,omitempty"`
	PostedDate            string                 `json:"PostedDate,omitempty"`
	RentalChargeList      []ChargeComponent      `json:"RentalChargeList,omitempty"`
	RentalFeeList         []FeeComponent         `json:"RentalFeeList,omitempty"`
	MarketplaceName       string                 `json:"MarketplaceName,omitempty"`
	RentalInitialValue    Currency               `json:"RentalInitialValue,omitempty"`
	RentalReimbursement   Currency               `json:"RentalReimbursement,omitempty"`
	RentalTaxWithheldList []TaxWithheldComponent `json:"RentalTaxWithheldList,omitempty"`
}

type ProductAdsPaymentEvent struct {
	PostedDate       string   `json:"PostedDate,omitempty"`
	TransactionType  string   `json:"TransactionType,omitempty"`
	InvoiceId        string   `json:"InvoiceId,omitempty"`
	BaseValue        Currency `json:"BaseValue,omitempty"`
	TaxValue         Currency `json:"TaxValue,omitempty"`
	TransactionValue Currency `json:"TransactionValue,omitempty"`
}

type ServiceFeeEvent struct {
	AmazonOrderId  string         `json:"AmazonOrderId,omitempty"`
	FeeReason      string         `json:"FeeReason,omitempty"`
	FeeList        []FeeComponent `json:"FeeList,omitempty"`
	SellerSKU      string         `json:"SellerSKU,omitempty"`
	FnSKU          string         `json:"FnSKU,omitempty"`
	FeeDescription string         `json:"FeeDescription,omitempty"`
	ASIN           string         `json:"ASIN,omitempty"`
}

type SellerDealPaymentEvent struct {
	PostedDate      string   `json:"PostedDate,omitempty"`
	DealId          string   `json:"DealId,omitempty"`
	DealDescription string   `json:"DealDescription,omitempty"`
	EventType       string   `json:"EventType,omitempty"`
	FeeType         string   `json:"FeeType,omitempty"`
	FeeAmount       Currency `json:"FeeAmount,omitempty"`
	TaxAmount       Currency `json:"TaxAmount,omitempty"`
	TotalAmount     Currency `json:"TotalAmount,omitempty"`
}

type DebtRecoveryEvent struct {
	DebtRecoveryType     string             `json:"DebtRecoveryType,omitempty"`
	RecoveryAmount       Currency           `json:"RecoveryAmount,omitempty"`
	OverPaymentCredit    Currency           `json:"OverPaymentCredit,omitempty"`
	DebtRecoveryItemList []DebtRecoveryItem `json:"DebtRecoveryItemList,omitempty"`
	ChargeInstrumentList []ChargeInstrument `json:"ChargeInstrumentList,omitempty"`
}

type DebtRecoveryItem struct {
	RecoveryAmount Currency `json:"RecoveryAmount,omitempty"`
	OriginalAmount Currency `json:"OriginalAmount,omitempty"`
	GroupBeginDate string   `json:"GroupBeginDate,omitempty"`
	GroupEndDate   string   `json:"GroupEndDate,omitempty"`
}

type ChargeInstrument struct {
	Description string   `json:"Description,omitempty"`
	Tail        string   `json:"Tail,omitempty"`
	Amount      Currency `json:"Amount,omitempty"`
}

type LoanServicingEvent struct {
	LoanAmount              Currency `json:"LoanAmount,omitempty"`
	SourceBusinessEventType string   `json:"SourceBusinessEventType,omitempty"`
}

type AdjustmentEvent struct {
	AdjustmentType     string           `json:"AdjustmentType,omitempty"`
	PostedDate         string           `json:"PostedDate,omitempty"`
	AdjustmentAmount   Currency         `json:"AdjustmentAmount,omitempty"`
	AdjustmentItemList []AdjustmentItem `json:"AdjustmentItemList,omitempty"`
}

type AdjustmentItem struct {
	Quantity           string   `json:"Quantity,omitempty"`
	PerUnitAmount      Currency `json:"PerUnitAmount,omitempty"`
	TotalAmount        Currency `json:"TotalAmount,omitempty"`
	SellerSKU          string   `json:"SellerSKU,omitempty"`
	FnSKU              string   `json:"FnSKU,omitempty"`
	ProductDescription string   `json:"ProductDescription,omitempty"`
	ASIN               string   `json:"ASIN,omitempty"`
}

type SAFETReimbursementEvent struct {
	PostedDate                 string                   `json:"PostedDate,omitempty"`
	SAFETClaimId               string                   `json:"SAFETClaimId,omitempty"`
	ReimbursedAmount           Currency                 `json:"ReimbursedAmount,omitempty"`
	ReasonCode                 string                   `json:"ReasonCode,omitempty"`
	SAFETReimbursementItemList []SAFETReimbursementItem `json:"SAFETReimbursementItemList,omitempty"`
}

type SAFETReimbursementItem struct {
	ItemChargeList     []ChargeComponent `json:"ItemChargeList,omitempty"`
	ProductDescription string            `json:"ProductDescription,omitempty"`
	Quantity           string            `json:"Quantity,omitempty"`
}

type SellerReviewEnrollmentPaymentEvent struct {
	PostedDate      string          `json:"PostedDate,omitempty"`
	EnrollmentId    string          `json:"EnrollmentId,omitempty"`
	ParentASIN      string          `json:"ParentASIN,omitempty"`
	FeeComponent    FeeComponent    `json:"FeeComponent,omitempty"`
	ChargeComponent ChargeComponent `json:"ChargeComponent,omitempty"`
	TotalAmount     Currency        `json:"TotalAmount,omitempty"`
}

type FBALiquidationEvent struct {
	PostedDate                string   `json:"PostedDate,omitempty"`
	OriginalRemovalOrderId    string   `json:"OriginalRemovalOrderId,omitempty"`
	LiquidationProceedsAmount Currency `json:"LiquidationProceedsAmount,omitempty"`
	LiquidationFeeAmount      Currency `json:"LiquidationFeeAmount,omitempty"`
}

type CouponPaymentEvent struct {
	PostedDate              string          `json:"PostedDate,omitempty"`
	CouponId                string          `json:"CouponId,omitempty"`
	SellerCouponDescription string          `json:"SellerCouponDescription,omitempty"`
	ClipOrRedemptionCount   int             `json:"ClipOrRedemptionCount,omitempty"`
	PaymentEventId          string          `json:"PaymentEventId,omitempty"`
	FeeComponent            FeeComponent    `json:"FeeComponent,omitempty"`
	ChargeComponent         ChargeComponent `json:"ChargeComponent,omitempty"`
	TotalAmount             Currency        `json:"TotalAmount,omitempty"`
}

type ImagingServicesFeeEvent struct {
	ImagingRequestBillingItemID string         `json:"ImagingRequestBillingItemID,omitempty"`
	ASIN                        string         `json:"ASIN,omitempty"`
	PostedDate                  string         `json:"PostedDate,omitempty"`
	FeeList                     []FeeComponent `json:"FeeList,omitempty"`
}

type NetworkComminglingTransactionEvent struct {
	TransactionType    string   `json:"TransactionType,omitempty"`
	PostedDate         string   `json:"PostedDate,omitempty"`
	NetCoTransactionID string   `json:"NetCoTransactionID,omitempty"`
	SwapReason         string   `json:"SwapReason,omitempty"`
	ASIN               string   `json:"ASIN,omitempty"`
	MarketplaceId      string   `json:"MarketplaceId,omitempty"`
	TaxExclusiveAmount Currency `json:"TaxExclusiveAmount,omitempty"`
	TaxAmount          Currency `json:"TaxAmount,omitempty"`
}

type AffordabilityExpenseEvent struct {
	AmazonOrderId   string   `json:"AmazonOrderId,omitempty"`
	PostedDate      string   `json:"PostedDate,omitempty"`
	MarketplaceId   string   `json:"MarketplaceId,omitempty"`
	TransactionType string   `json:"TransactionType,omitempty"`
	BaseExpense     Currency `json:"BaseExpense,omitempty"`
	TaxTypeIGST     Currency `json:"TaxTypeIGST,omitempty"`
	TaxTypeCGST     Currency `json:"TaxTypeCGST,omitempty"`
	TaxTypeSGST     Currency `json:"TaxTypeSGST,omitempty"`
	TotalExpense    Currency `json:"TotalExpense,omitempty"`
}

type RemovalShipmentEvent struct {
	PostedDate              string                `json:"PostedDate,omitempty"`
	OrderId                 string                `json:"OrderId,omitempty"`
	TransactionType         string                `json:"TransactionType,omitempty"`
	RemovalShipmentItemList []RemovalShipmentItem `json:"RemovalShipmentItemList,omitempty"`
}

type RemovalShipmentItem struct {
	RemovalShipmentItemId string   `json:"RemovalShipmentItemId,omitempty"`
	TaxCollectionModel    string   `json:"TaxCollectionModel,omitempty"`
	FulfillmentNetworkSKU string   `json:"FulfillmentNetworkSKU,omitempty"`
	Quantity              int      `json:"Quantity,omitempty"`
	Revenue               Currency `json:"Revenue,omitempty"`
	FeeAmount             Currency `json:"FeeAmount,omitempty"`
	TaxAmount             Currency `json:"TaxAmount,omitempty"`
	TaxWithheld           Currency `json:"TaxWithheld,omitempty"`
}

type RemovalShipmentAdjustmentEvent struct {
	PostedDate                        string                `json:"PostedDate,omitempty"`
	AdjustmentEventId                 string                `json:"AdjustmentEventId,omitempty"`
	MerchantOrderId                   string                `json:"MerchantOrderId,omitempty"`
	OrderId                           string                `json:"OrderId,omitempty"`
	TransactionType                   string                `json:"TransactionType,omitempty"`
	RemovalShipmentItemAdjustmentList []RemovalShipmentItem `json:"RemovalShipmentItemAdjustmentList,omitempty"`
}

type TrialShipmentEvent struct {
	AmazonOrderId         string         `json:"AmazonOrderId,omitempty"`
	FinancialEventGroupId string         `json:"FinancialEventGroupId,omitempty"`
	PostedDate            string         `json:"PostedDate,omitempty"`
	SKU                   string         `json:"SKU,omitempty"`
	FeeList               []FeeComponent `json:"FeeList,omitempty"`
}

type TDSReimbursementEvent struct {
	PostedDate       string   `json:"PostedDate,omitempty"`
	TdsOrderId       string   `json:"TdsOrderId,omitempty"`
	ReimbursedAmount Currency `json:"ReimbursedAmount,omitempty"`
}

type AdhocDisbursementEvent struct {
	TransactionType         string   `json:"TransactionType,omitempty"`
	PostedDate              string   `json:"PostedDate,omitempty"`
	SourceOrderId           string   `json:"SourceOrderId,omitempty"`
	SourceOrderItemId       string   `json:"SourceOrderItemId,omitempty"`
	AdhocDisbursementAmount Currency `json:"AdhocDisbursementAmount,omitempty"`
}

type TaxWithholdingEvent struct {
	PostedDate           string                 `json:"PostedDate,omitempty"`
	BaseAmount           Currency               `json:"BaseAmount,omitempty"`
	WithheldAmount       Currency               `json:"WithheldAmount,omitempty"`
	TaxWithholdingPeriod TaxWithholdingPeriod   `json:"TaxWithholdingPeriod,omitempty"`
	TaxesWithheld        []TaxWithheldComponent `json:"TaxesWithheld,omitempty"`
}

type TaxWithholdingPeriod struct {
	StartDate string `json:"StartDate,omitempty"`
	EndDate   string `json:"EndDate,omitempty"`
}

type ChargeRefundEvent struct {
	PostedDate   string   `json:"PostedDate,omitempty"`
	ReasonCode   string   `json:"ReasonCode,omitempty"`
	ReferenceId  string   `json:"ReferenceId,omitempty"`
	RefundAmount Currency `json:"RefundAmount,omitempty"`
}

type FailedAdhocDisbursementEvent struct {
	FundsRequestType string   `json:"FundsRequestType,omitempty"`
	PrincipalAmount  Currency `json:"PrincipalAmount,omitempty"`
	EscrowAmount     Currency `json:"EscrowAmount,omitempty"`
}

type ValueAddedServiceChargeEvent struct {
	TransactionType   string   `json:"TransactionType,omitempty"`
	PostedDate        string   `json:"PostedDate,omitempty"`
	Description       string   `json:"Description,omitempty"`
	TransactionAmount Currency `json:"TransactionAmount,omitempty"`
}

type CapacityReservationBillingEvent struct {
	TransactionType   string   `json:"TransactionType,omitempty"`
	PostedDate        string   `json:"PostedDate,omitempty"`
	Description       string   `json:"Description,omitempty"`
	TransactionAmount Currency `json:"TransactionAmount,omitempty"`
}

type CreateInboundShipmentPlanEvent struct {
	ShipmentPlanId string         `json:"ShipmentPlanId,omitempty"`
	PostedDate     string         `json:"PostedDate,omitempty"`
	FeeList        []FeeComponent `json:"FeeList,omitempty"`
}

type ShippingLabelEvent struct {
	ShipmentId string         `json:"ShipmentId,omitempty"`
	PostedDate string         `json:"PostedDate,omitempty"`
	FeeList    []FeeComponent `json:"FeeList,omitempty"`
}

// Order Items Response Structures
type OrderItemsResponse struct {
	Payload struct {
		AmazonOrderId string      `json:"AmazonOrderId"`
		OrderItems    []OrderItem `json:"OrderItems"`
		NextToken     string      `json:"NextToken,omitempty"`
	} `json:"payload"`
}

type OrderItem struct {
	ASIN                     string                 `json:"ASIN,omitempty"`
	SellerSKU                string                 `json:"SellerSKU,omitempty"`
	OrderItemId              string                 `json:"OrderItemId,omitempty"`
	Title                    string                 `json:"Title,omitempty"`
	QuantityOrdered          int                    `json:"QuantityOrdered,omitempty"`
	QuantityShipped          int                    `json:"QuantityShipped,omitempty"`
	ProductInfo              ProductInfo            `json:"ProductInfo,omitempty"`
	PointsGranted            PointsGranted          `json:"PointsGranted,omitempty"`
	ItemPrice                Money                  `json:"ItemPrice,omitempty"`
	ShippingPrice            Money                  `json:"ShippingPrice,omitempty"`
	ItemTax                  Money                  `json:"ItemTax,omitempty"`
	ShippingTax              Money                  `json:"ShippingTax,omitempty"`
	ShippingDiscount         Money                  `json:"ShippingDiscount,omitempty"`
	ShippingDiscountTax      Money                  `json:"ShippingDiscountTax,omitempty"`
	PromotionDiscount        Money                  `json:"PromotionDiscount,omitempty"`
	PromotionDiscountTax     Money                  `json:"PromotionDiscountTax,omitempty"`
	PromotionIds             []string               `json:"PromotionIds,omitempty"`
	CODFee                   Money                  `json:"CODFee,omitempty"`
	CODFeeDiscount           Money                  `json:"CODFeeDiscount,omitempty"`
	IsGift                   bool                   `json:"IsGift,omitempty"`
	ConditionNote            string                 `json:"ConditionNote,omitempty"`
	ConditionId              string                 `json:"ConditionId,omitempty"`
	ConditionSubtypeId       string                 `json:"ConditionSubtypeId,omitempty"`
	ScheduledDeliveryStartDate string               `json:"ScheduledDeliveryStartDate,omitempty"`
	ScheduledDeliveryEndDate string                 `json:"ScheduledDeliveryEndDate,omitempty"`
	PriceDesignation         string                 `json:"PriceDesignation,omitempty"`
	TaxCollection            TaxCollection          `json:"TaxCollection,omitempty"`
	SerialNumberRequired     bool                   `json:"SerialNumberRequired,omitempty"`
	IsTransparency           bool                   `json:"IsTransparency,omitempty"`
	IossNumber               string                 `json:"IossNumber,omitempty"`
	StoreChainStoreId        string                 `json:"StoreChainStoreId,omitempty"`
	DeemedResellerCategory   string                 `json:"DeemedResellerCategory,omitempty"`
	BuyerInfo                BuyerInfo              `json:"BuyerInfo,omitempty"`
}

type ProductInfo struct {
	NumberOfItems int `json:"NumberOfItems,omitempty"`
}

type PointsGranted struct {
	PointsNumber      int   `json:"PointsNumber,omitempty"`
	PointsMonetaryValue Money `json:"PointsMonetaryValue,omitempty"`
}

type Money struct {
	CurrencyCode string  `json:"CurrencyCode,omitempty"`
	Amount       string  `json:"Amount,omitempty"`
}

type TaxCollection struct {
	Model             string `json:"Model,omitempty"`
	ResponsibleParty  string `json:"ResponsibleParty,omitempty"`
}

type BuyerInfo struct {
	BuyerCustomizedInfo BuyerCustomizedInfo `json:"BuyerCustomizedInfo,omitempty"`
	GiftWrapPrice       Money               `json:"GiftWrapPrice,omitempty"`
	GiftWrapTax         Money               `json:"GiftWrapTax,omitempty"`
	GiftMessageText     string              `json:"GiftMessageText,omitempty"`
	GiftWrapLevel       string              `json:"GiftWrapLevel,omitempty"`
}

type BuyerCustomizedInfo struct {
	CustomizedURL string `json:"CustomizedURL,omitempty"`
}

// Financial Event Groups Response Structures
type FinancialEventGroupsResponse struct {
	Payload struct {
		FinancialEventGroupList []FinancialEventGroup `json:"FinancialEventGroupList"`
		NextToken               string                `json:"NextToken,omitempty"`
	} `json:"payload"`
}

type FinancialEventGroup struct {
	FinancialEventGroupId    string   `json:"FinancialEventGroupId,omitempty"`
	ProcessingStatus         string   `json:"ProcessingStatus,omitempty"`
	FundTransferStatus       string   `json:"FundTransferStatus,omitempty"`
	OriginalTotal            Currency `json:"OriginalTotal,omitempty"`
	ConvertedTotal           Currency `json:"ConvertedTotal,omitempty"`
	FundTransferDate         string   `json:"FundTransferDate,omitempty"`
	TraceId                  string   `json:"TraceId,omitempty"`
	AccountTail              string   `json:"AccountTail,omitempty"`
	BeginningBalance         Currency `json:"BeginningBalance,omitempty"`
	FinancialEventGroupStart string   `json:"FinancialEventGroupStart,omitempty"`
	FinancialEventGroupEnd   string   `json:"FinancialEventGroupEnd,omitempty"`
}

// Simplified Financial Event Structure - Flat format with amounts only
type SimplifiedFinancialEvent struct {
	EventType       string  `json:"event_type"`
	PostedDate      string  `json:"posted_date,omitempty"`
	AmazonOrderId   string  `json:"amazon_order_id,omitempty"`
	SellerOrderId   string  `json:"seller_order_id,omitempty"`
	MarketplaceName string  `json:"marketplace_name,omitempty"`
	ASIN            string  `json:"asin,omitempty"`
	SellerSKU       string  `json:"seller_sku,omitempty"`
	TransactionType string  `json:"transaction_type,omitempty"`
	Amount          float64 `json:"amount"`
	CurrencyCode    string  `json:"currency_code"`
	FeeType         string  `json:"fee_type,omitempty"`
	ChargeType      string  `json:"charge_type,omitempty"`
	Description     string  `json:"description,omitempty"`
	Quantity        int     `json:"quantity,omitempty"`
	ReasonCode      string  `json:"reason_code,omitempty"`
	ReferenceId     string  `json:"reference_id,omitempty"`
}

type AmazonTool struct {
	tools.Tool
	AmazonAccount AmazonAccount `json:"amazon_account"`
	AuthName      string        `json:"auth_name"`
}
type amazonToolWithToken struct {
	tools.Tool
	AmazonAccount amazonAccountWithToken
	AuthName      string
}

const (
	SellerBaseUrl = "https://sellingpartnerapi-eu.amazon.com"
)

func (amazonTool *AmazonTool) GetActionsList() []string {
	actions := []string{}
	actions = append(actions, GetOrders, GetOrderItems, GetFinancialEvents, GetFinancialEventGroups, Login, RenewToken, GetSsoUrl, StopAutoRenew)
	return actions
}

func (amazonTool *AmazonTool) GetSpec() tools.Tooling {
	return amazonTool
}

func (amazonTool *AmazonTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &amazonTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (amazonTool *AmazonTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("AmazonTool Execute - Start")
	switch actionName {
	case GetOrders:
		return amazonTool.GetOrders(ctx, params)
	case GetOrderItems:
		return amazonTool.GetOrderItems(ctx, params)
	case GetFinancialEvents:
		return amazonTool.GetFinancialEvents(ctx, params)
	case GetFinancialEventGroups:
		return amazonTool.GetFinancialEventGroups(ctx, params)
	case Login:
		return amazonTool.Login(ctx, projectId, tenantId, params, "")
	case RenewToken:
		return amazonTool.RenewToken(ctx, projectId, tenantId, params)
	case GetSsoUrl:
		return amazonTool.GetSsoUrl(ctx, projectId, tenantId, params)
	case StopAutoRenew:
		return amazonTool.StopAutoRenew(ctx, projectId, tenantId, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}
}

func (amazonTool *AmazonTool) GetOrders(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetOrders Execute - Start")

	nextToken := ""

	// Convert params to query parameters
	queryParams := map[string]string{}
	for k, v := range params {
		queryParams[k] = v.(string)
	}

	// Call recursively to get all orders
	consolidatedResponse, err := amazonTool.getOrdersRecursive(ctx, queryParams, nextToken)
	if err != nil {
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["orders"] = consolidatedResponse
	return toolResult, false, nil
}

func (amazonTool *AmazonTool) getOrdersRecursive(ctx context.Context, queryParams map[string]string, nextToken string) ([]interface{}, error) {
	logs.WithContext(ctx).Debug("getOrdersRecursive Execute - Start")
	var allOrders []interface{}
	url := fmt.Sprint(SellerBaseUrl, "/orders/v0/orders")
	headers := http.Header{}
	headers.Set("x-amz-access-token", amazonTool.AmazonAccount.AccessToken)
	headers.Set("x-amz-date", time.Now().UTC().Format(time.RFC3339))
	headers.Set("user-agent", amazonTool.AmazonAccount.UserAgent)
	headers.Set("content-type", "application/json")

	// Prepare query parameters based on whether NextToken is provided
	currentQueryParams := make(map[string]string)
	if nextToken != "" {
		// When NextToken is provided, only pass the NextToken parameter
		currentQueryParams["NextToken"] = nextToken
	} else {
		// For the first call, pass all original parameters
		for k, v := range queryParams {
			currentQueryParams[k] = v
		}
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, currentQueryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}

	// Parse response to extract orders and next token
	responseMap, ok := res.(map[string]interface{})
	if !ok {
		logs.WithContext(ctx).Error("Response is not a map")
		return nil, errors.New("invalid response format")
	}

	// Extract orders from current response
	if responsePayload, exists := responseMap["payload"]; exists {
		if payloadMap, ok := responsePayload.(map[string]interface{}); ok {
			if ordersData, exists := payloadMap["Orders"]; exists {
				if ordersList, ok := ordersData.([]interface{}); ok {
					allOrders = append(allOrders, ordersList...)
				}
			}
			if nt, exists := payloadMap["NextToken"]; exists {
				nextToken = nt.(string)
				logs.WithContext(ctx).Info("Found NextToken: %s, making recursive call")
				// Recursive call with NextToken
				nextedOrders, err := amazonTool.getOrdersRecursive(ctx, queryParams, nextToken)
				if err != nil {
					return nil, err
				}
				allOrders = append(allOrders, nextedOrders...)
			}
		}
	}

	// No more NextToken, return consolidated response
	logs.WithContext(ctx).Debug(fmt.Sprintf("No more NextToken found. Total orders collected: %d", len(allOrders)))

	return allOrders, nil
}

func (amazonTool *AmazonTool) GetOrderItems(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetOrderItems Execute - Start")

	// Extract order_id from params
	orderId, exists := params["order_id"]
	if !exists {
		return nil, false, errors.New("order_id parameter is required")
	}
	orderIdStr := orderId.(string)
	if orderIdStr == "" {
		return nil, false, errors.New("order_id parameter cannot be empty")
	}

	// Build URL with order_id in path
	url := fmt.Sprint(SellerBaseUrl, "/orders/v0/orders/", orderIdStr, "/orderItems")
	headers := http.Header{}
	headers.Set("x-amz-access-token", amazonTool.AmazonAccount.AccessToken)
	headers.Set("x-amz-date", time.Now().UTC().Format(time.RFC3339))
	headers.Set("user-agent", amazonTool.AmazonAccount.UserAgent)
	headers.Set("content-type", "application/json")

	// Prepare query parameters - exclude order_id since it's in the path
	queryParams := make(map[string]string)
	for k, v := range params {
		if k != "order_id" {
			queryParams[k] = v.(string)
		}
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	// Parse response
	responseMap, ok := res.(map[string]interface{})
	if !ok {
		logs.WithContext(ctx).Error("Response is not a map")
		return nil, false, errors.New("invalid response format")
	}

	toolResult = make(map[string]interface{})
	
	// Extract order items from response
	if responsePayload, exists := responseMap["payload"]; exists {
		if payloadMap, ok := responsePayload.(map[string]interface{}); ok {
			if orderItems, exists := payloadMap["OrderItems"]; exists {
				toolResult["order_items"] = orderItems
			}
			if amazonOrderId, exists := payloadMap["AmazonOrderId"]; exists {
				toolResult["amazon_order_id"] = amazonOrderId
			}
		}
	}

	return toolResult, false, nil
}

func (amazonTool *AmazonTool) GetFinancialEventGroups(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetFinancialEventGroups Execute - Start")

	nextToken := ""

	// Convert params to query parameters
	queryParams := map[string]string{}
	for k, v := range params {
		queryParams[k] = v.(string)
	}

	// Call recursively to get all financial event groups
	consolidatedResponse, err := amazonTool.getFinancialEventGroupsRecursive(ctx, queryParams, nextToken)
	if err != nil {
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["financial_event_groups"] = consolidatedResponse
	return toolResult, false, nil
}

func (amazonTool *AmazonTool) getFinancialEventGroupsRecursive(ctx context.Context, queryParams map[string]string, nextToken string) ([]interface{}, error) {
	logs.WithContext(ctx).Debug("getFinancialEventGroupsRecursive Execute - Start")
	var allGroups []interface{}
	url := fmt.Sprint(SellerBaseUrl, "/finances/v0/financialEventGroups")
	headers := http.Header{}
	headers.Set("x-amz-access-token", amazonTool.AmazonAccount.AccessToken)
	headers.Set("x-amz-date", time.Now().UTC().Format(time.RFC3339))
	headers.Set("user-agent", amazonTool.AmazonAccount.UserAgent)
	headers.Set("content-type", "application/json")

	// Prepare query parameters based on whether NextToken is provided
	currentQueryParams := make(map[string]string)
	if nextToken != "" {
		// When NextToken is provided, only pass the NextToken parameter
		currentQueryParams["NextToken"] = nextToken
	} else {
		// For the first call, pass all original parameters
		for k, v := range queryParams {
			currentQueryParams[k] = v
		}
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, currentQueryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}

	// Parse response to extract financial event groups and next token
	responseMap, ok := res.(map[string]interface{})
	if !ok {
		logs.WithContext(ctx).Error("Response is not a map")
		return nil, errors.New("invalid response format")
	}

	// Extract financial event groups from current response
	if responsePayload, exists := responseMap["payload"]; exists {
		if payloadMap, ok := responsePayload.(map[string]interface{}); ok {
			if groupsData, exists := payloadMap["FinancialEventGroupList"]; exists {
				if groupsList, ok := groupsData.([]interface{}); ok {
					allGroups = append(allGroups, groupsList...)
				}
			}
			if nt, exists := payloadMap["NextToken"]; exists {
				nextToken = nt.(string)
				logs.WithContext(ctx).Info("Found NextToken: %s, making recursive call")
				// Recursive call with NextToken
				nextGroups, err := amazonTool.getFinancialEventGroupsRecursive(ctx, queryParams, nextToken)
				if err != nil {
					return nil, err
				}
				allGroups = append(allGroups, nextGroups...)
			}
		}
	}

	// No more NextToken, return consolidated response
	logs.WithContext(ctx).Debug(fmt.Sprintf("No more NextToken found. Total financial event groups collected: %d", len(allGroups)))

	return allGroups, nil
}

func (amazonTool *AmazonTool) GetFinancialEvents(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetFinancialEvents Execute - Start")

	nextToken := ""

	// Convert params to query parameters
	queryParams := map[string]string{}
	for k, v := range params {
		queryParams[k] = v.(string)
	}

	// Call recursively to get all financial events with structured merging
	consolidatedFinancialEvents, err := amazonTool.getFinancialEventsRecursive(ctx, queryParams, nextToken)
	if err != nil {
		return nil, false, err
	}

	toolResult = make(map[string]interface{})

	// Check output_format parameter
	outputFormat := "simplified" // default
	if params["output_format"] != nil {
		outputFormat = params["output_format"].(string)
	}

	if outputFormat == "raw" {
		// Return complete structured financial events
		toolResult["financial_events"] = consolidatedFinancialEvents
	} else {
		// Return simplified flat structure (default)
		simplifiedEvents := amazonTool.convertToSimplifiedEvents(consolidatedFinancialEvents)
		toolResult["financial_events"] = simplifiedEvents
	}

	return toolResult, false, nil
}

func (amazonTool *AmazonTool) getFinancialEventsRecursive(ctx context.Context, queryParams map[string]string, nextToken string) (*FinancialEvents, error) {
	logs.WithContext(ctx).Debug("getFinancialEventsRecursive Execute - Start")

	// Initialize consolidated financial events structure
	consolidatedEvents := &FinancialEvents{}

	return amazonTool.getFinancialEventsRecursiveHelper(ctx, queryParams, nextToken, consolidatedEvents)
}

func (amazonTool *AmazonTool) getFinancialEventsRecursiveHelper(ctx context.Context, queryParams map[string]string, nextToken string, consolidatedEvents *FinancialEvents) (*FinancialEvents, error) {
	url := fmt.Sprint(SellerBaseUrl, "/finances/v0/financialEvents")
	headers := http.Header{}
	headers.Set("x-amz-access-token", amazonTool.AmazonAccount.AccessToken)
	headers.Set("x-amz-date", time.Now().UTC().Format(time.RFC3339))
	headers.Set("user-agent", amazonTool.AmazonAccount.UserAgent)
	headers.Set("content-type", "application/json")

	// Prepare query parameters based on whether NextToken is provided
	currentQueryParams := make(map[string]string)
	if nextToken != "" {
		// When NextToken is provided, only pass the NextToken parameter
		currentQueryParams["NextToken"] = nextToken
	} else {
		// For the first call, pass all original parameters except output_format
		for k, v := range queryParams {
			if k != "output_format" {
				currentQueryParams[k] = v
			}
		}
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, currentQueryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}

	// Parse response to extract financial events and next token
	responseMap, ok := res.(map[string]interface{})
	if !ok {
		logs.WithContext(ctx).Error("Response is not a map")
		return nil, errors.New("invalid response format")
	}

	// Extract financial events from current response
	if responsePayload, exists := responseMap["payload"]; exists {
		if payloadMap, ok := responsePayload.(map[string]interface{}); ok {
			if financialEventsData, exists := payloadMap["FinancialEvents"]; exists {
				// Parse current page's financial events
				eventsBytes, err := json.Marshal(financialEventsData)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal financial events: %w", err)
				}

				var currentPageEvents FinancialEvents
				err = json.Unmarshal(eventsBytes, &currentPageEvents)
				if err != nil {
					return nil, fmt.Errorf("failed to unmarshal financial events: %w", err)
				}

				// Merge current page events with consolidated events
				amazonTool.mergeFinancialEvents(consolidatedEvents, &currentPageEvents)
			}

			if nt, exists := payloadMap["NextToken"]; exists {
				nextToken = nt.(string)
				logs.WithContext(ctx).Info("Found NextToken: %s, making recursive call")
				// Recursive call with NextToken
				return amazonTool.getFinancialEventsRecursiveHelper(ctx, queryParams, nextToken, consolidatedEvents)
			}
		}
	}

	// No more NextToken, return consolidated response
	logs.WithContext(ctx).Debug("No more NextToken found. Consolidated financial events completed")
	return consolidatedEvents, nil
}

func (amazonTool *AmazonTool) GetSsoUrl(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetSsoUrl Execute - Start")

	eruauthUrl := ctx.Value("eruauthbaseurl").(string)
	url := fmt.Sprint(eruauthUrl, "/", projectId, "/", amazonTool.AuthName, "/getssourl")
	logs.WithContext(ctx).Info(fmt.Sprint("url: ", url))
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	qParams := make(map[string]string)
	if params["state"] != nil {
		qParams["state"] = params["state"].(string)
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, qParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	toolResultOk := false
	toolResult, toolResultOk = res.(map[string]interface{})
	if !toolResultOk {
		err = errors.New("toolResult is not a map")
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	logs.WithContext(ctx).Info(fmt.Sprint("toolResult: ", toolResult))
	return toolResult, false, nil
}
func (amazonTool *AmazonTool) RenewToken(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	params["refresh_token"] = amazonTool.AmazonAccount.RefreshToken
	return amazonTool.Login(ctx, projectId, tenantId, params, "/renew")
}

func (amazonTool *AmazonTool) Login(ctx context.Context, projectId string, tenantId string, params map[string]interface{}, renewStr string) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("Login Execute - Start")
	if amazonTool.AuthName == "" {
		err = errors.New("auth name is required")
		logs.Err(ctx, err, "")
		return nil, false, err
	}
	eruauthUrl := ctx.Value("eruauthbaseurl").(string)
	url := fmt.Sprint(eruauthUrl, "/", projectId, "/", amazonTool.AuthName, "/idptoken", renewStr)
	logs.WithContext(ctx).Info(fmt.Sprint("url: ", url))
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, params)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	var amazonTokens AmazonTokens
	resBytes, _ := json.Marshal(res)
	err = json.Unmarshal(resBytes, &amazonTokens)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	err = json.Unmarshal(resBytes, &amazonTokens)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	err = amazonTool.saveTenantSecret(ctx, projectId, tenantId, fmt.Sprint(amazonTool.AuthName, "_access_token"), amazonTokens.AccessToken)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	err = amazonTool.saveTenantSecret(ctx, projectId, tenantId, fmt.Sprint(amazonTool.AuthName, "_refresh_token"), amazonTokens.RefreshToken)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	amazonTool.AmazonAccount.TokenExpirationDateTime = time.Now().UTC().Add(time.Duration(amazonTokens.ExpiresIn) * time.Second).Format(time.RFC3339)
	persistStore = true

	if amazonTool.Tool.Hooks.ARRT != "" {
		hookBody := map[string]interface{}{
			"Vars": map[string]interface{}{
				"Body": map[string]interface{}{
					"tool_name": amazonTool.ToolName,
					"tenant_id": tenantId,
				},
				"OrgBody": map[string]interface{}{
					"tool_name": amazonTool.ToolName,
					"tenant_id": tenantId,
				},
			},
			"ReqVars": map[string]interface{}{},
			"ResVars": map[string]interface{}{},
		}
		hookBodyBytes, err := json.Marshal(hookBody)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, persistStore, err
		}
		jobName := fmt.Sprint(amazonTool.Tool.Hooks.ARRT, "_", tenantId)
		err = amazonTool.Scheduler.Unschedule(ctx, "", jobName)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			//continue even if unschedule fails
		}

		schedulerCommand := fmt.Sprint("CALL schedule_procedure('", amazonTool.Tool.Hooks.ARRT, "','", string(hookBodyBytes), "','", amazonTool.Scheduler.GetSchedulerName(), "')")

		cronStr := utils.GetCronStr(ctx, time.Now().UTC().Add(1*time.Hour))
		jobId, err := amazonTool.Scheduler.Schedule(ctx, jobName, schedulerCommand, cronStr)
		if err != nil {
			return nil, persistStore, err
		}
		logs.WithContext(ctx).Info(fmt.Sprint("jobId: ", jobId))
	}
	toolResult = make(map[string]interface{})
	toolResult["login_status"] = "success"
	return toolResult, persistStore, nil
}

func (amazonTool *AmazonTool) saveTenantSecret(ctx context.Context, projectId string, tenantId string, secretName string, secretValue string) (err error) {
	logs.WithContext(ctx).Debug("saveTenantSecret Execute - Start")
	eruaiport := ctx.Value("eruaiport").(string)
	url := fmt.Sprint("http://localhost:", eruaiport, "/store/", projectId, "/", tenantId, "/sm/set")
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	secretPost := make(map[string]interface{})
	secretInnerPost := make(map[string]interface{})
	secretInnerPost[secretName] = secretValue
	secretPost["secret_value"] = secretInnerPost
	_, _, _, _, err = utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, secretPost)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (amazonTool *AmazonTool) SetPrivateAttributes(ctx context.Context, realTool tools.Tooling) (err error) {
	amazonTool.AmazonAccount.AccessToken = "$SECRET_amazon_access_token"
	amazonTool.AmazonAccount.RefreshToken = "$SECRET_amazon_refresh_token"
	return nil
}

func (amazonTool *AmazonTool) GetBytes(ctx context.Context) ([]byte, error) {

	amazonToolWithToken := amazonToolWithToken{
		Tool: amazonTool.Tool,
		AmazonAccount: amazonAccountWithToken{
			UserAgent:               amazonTool.AmazonAccount.UserAgent,
			AccessToken:             amazonTool.AmazonAccount.AccessToken,
			RefreshToken:            amazonTool.AmazonAccount.RefreshToken,
			TokenExpirationDateTime: amazonTool.AmazonAccount.TokenExpirationDateTime,
		},
		AuthName: amazonTool.AuthName,
	}

	toolJson, err := json.Marshal(amazonToolWithToken)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}
func (amazonTool *AmazonTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	amazonToolWithToken := amazonToolWithToken{}
	err := json.Unmarshal(toolObjJson, &amazonToolWithToken)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	amazonTool = &AmazonTool{
		Tool: amazonToolWithToken.Tool,
		AmazonAccount: AmazonAccount{
			UserAgent:               amazonToolWithToken.AmazonAccount.UserAgent,
			AccessToken:             amazonToolWithToken.AmazonAccount.AccessToken,
			RefreshToken:            amazonToolWithToken.AmazonAccount.RefreshToken,
			TokenExpirationDateTime: amazonToolWithToken.AmazonAccount.TokenExpirationDateTime,
		},
		AuthName: amazonToolWithToken.AuthName,
	}
	return amazonTool, nil
}
func (amazonTool *AmazonTool) convertToSimplifiedEvents(financialEvents *FinancialEvents) []SimplifiedFinancialEvent {
	var simplified []SimplifiedFinancialEvent

	// Process Shipment Events
	for _, event := range financialEvents.ShipmentEventList {
		// Process order charges
		for _, charge := range event.OrderChargeList {
			if charge.ChargeAmount.CurrencyAmount != 0 {
				simplified = append(simplified, SimplifiedFinancialEvent{
					EventType:       "shipment_order_charge",
					PostedDate:      event.PostedDate,
					AmazonOrderId:   event.AmazonOrderId,
					SellerOrderId:   event.SellerOrderId,
					MarketplaceName: event.MarketplaceName,
					Amount:          charge.ChargeAmount.CurrencyAmount,
					CurrencyCode:    charge.ChargeAmount.CurrencyCode,
					ChargeType:      charge.ChargeType,
				})
			}
		}
		// Process shipment fees
		for _, fee := range event.ShipmentFeeList {
			if fee.FeeAmount.CurrencyAmount != 0 {
				simplified = append(simplified, SimplifiedFinancialEvent{
					EventType:       "shipment_fee",
					PostedDate:      event.PostedDate,
					AmazonOrderId:   event.AmazonOrderId,
					SellerOrderId:   event.SellerOrderId,
					MarketplaceName: event.MarketplaceName,
					Amount:          fee.FeeAmount.CurrencyAmount,
					CurrencyCode:    fee.FeeAmount.CurrencyCode,
					FeeType:         fee.FeeType,
				})
			}
		}
		// Process order fees
		for _, fee := range event.OrderFeeList {
			if fee.FeeAmount.CurrencyAmount != 0 {
				simplified = append(simplified, SimplifiedFinancialEvent{
					EventType:       "order_fee",
					PostedDate:      event.PostedDate,
					AmazonOrderId:   event.AmazonOrderId,
					SellerOrderId:   event.SellerOrderId,
					MarketplaceName: event.MarketplaceName,
					Amount:          fee.FeeAmount.CurrencyAmount,
					CurrencyCode:    fee.FeeAmount.CurrencyCode,
					FeeType:         fee.FeeType,
				})
			}
		}
		// Process shipment items
		for _, item := range event.ShipmentItemList {
			// Process item charges
			for _, charge := range item.ItemChargeList {
				if charge.ChargeAmount.CurrencyAmount != 0 {
					simplified = append(simplified, SimplifiedFinancialEvent{
						EventType:       "shipment_item_charge",
						PostedDate:      event.PostedDate,
						AmazonOrderId:   event.AmazonOrderId,
						SellerOrderId:   event.SellerOrderId,
						MarketplaceName: event.MarketplaceName,
						SellerSKU:       item.SellerSKU,
						Amount:          charge.ChargeAmount.CurrencyAmount,
						CurrencyCode:    charge.ChargeAmount.CurrencyCode,
						ChargeType:      charge.ChargeType,
						Quantity:        item.QuantityShipped,
					})
				}
			}
			// Process item fees
			for _, fee := range item.ItemFeeList {
				if fee.FeeAmount.CurrencyAmount != 0 {
					simplified = append(simplified, SimplifiedFinancialEvent{
						EventType:       "shipment_item_fee",
						PostedDate:      event.PostedDate,
						AmazonOrderId:   event.AmazonOrderId,
						SellerOrderId:   event.SellerOrderId,
						MarketplaceName: event.MarketplaceName,
						SellerSKU:       item.SellerSKU,
						Amount:          fee.FeeAmount.CurrencyAmount,
						CurrencyCode:    fee.FeeAmount.CurrencyCode,
						FeeType:         fee.FeeType,
						Quantity:        item.QuantityShipped,
					})
				}
			}
			// Process promotions
			for _, promotion := range item.PromotionList {
				if promotion.PromotionAmount.CurrencyAmount != 0 {
					simplified = append(simplified, SimplifiedFinancialEvent{
						EventType:       "promotion",
						PostedDate:      event.PostedDate,
						AmazonOrderId:   event.AmazonOrderId,
						SellerOrderId:   event.SellerOrderId,
						MarketplaceName: event.MarketplaceName,
						SellerSKU:       item.SellerSKU,
						Amount:          promotion.PromotionAmount.CurrencyAmount,
						CurrencyCode:    promotion.PromotionAmount.CurrencyCode,
						Description:     promotion.PromotionType,
						ReferenceId:     promotion.PromotionId,
						Quantity:        item.QuantityShipped,
					})
				}
			}
		}
	}

	// Process Refund Events (similar structure to shipment events)
	for _, event := range financialEvents.RefundEventList {
		for _, charge := range event.OrderChargeList {
			if charge.ChargeAmount.CurrencyAmount != 0 {
				simplified = append(simplified, SimplifiedFinancialEvent{
					EventType:       "refund_order_charge",
					PostedDate:      event.PostedDate,
					AmazonOrderId:   event.AmazonOrderId,
					SellerOrderId:   event.SellerOrderId,
					MarketplaceName: event.MarketplaceName,
					Amount:          charge.ChargeAmount.CurrencyAmount,
					CurrencyCode:    charge.ChargeAmount.CurrencyCode,
					ChargeType:      charge.ChargeType,
				})
			}
		}
	}

	// Process Service Fee Events
	for _, event := range financialEvents.ServiceFeeEventList {
		for _, fee := range event.FeeList {
			if fee.FeeAmount.CurrencyAmount != 0 {
				simplified = append(simplified, SimplifiedFinancialEvent{
					EventType:     "service_fee",
					AmazonOrderId: event.AmazonOrderId,
					SellerSKU:     event.SellerSKU,
					ASIN:          event.ASIN,
					Amount:        fee.FeeAmount.CurrencyAmount,
					CurrencyCode:  fee.FeeAmount.CurrencyCode,
					FeeType:       fee.FeeType,
					Description:   event.FeeDescription,
					ReasonCode:    event.FeeReason,
				})
			}
		}
	}

	// Process Product Ads Payment Events
	for _, event := range financialEvents.ProductAdsPaymentEventList {
		if event.TransactionValue.CurrencyAmount != 0 {
			simplified = append(simplified, SimplifiedFinancialEvent{
				EventType:       "product_ads_payment",
				PostedDate:      event.PostedDate,
				TransactionType: event.TransactionType,
				Amount:          event.TransactionValue.CurrencyAmount,
				CurrencyCode:    event.TransactionValue.CurrencyCode,
				ReferenceId:     event.InvoiceId,
			})
		}
	}

	// Process Adjustment Events
	for _, event := range financialEvents.AdjustmentEventList {
		if event.AdjustmentAmount.CurrencyAmount != 0 {
			simplified = append(simplified, SimplifiedFinancialEvent{
				EventType:    "adjustment",
				PostedDate:   event.PostedDate,
				Amount:       event.AdjustmentAmount.CurrencyAmount,
				CurrencyCode: event.AdjustmentAmount.CurrencyCode,
				Description:  event.AdjustmentType,
			})
		}
		// Process adjustment items
		for _, item := range event.AdjustmentItemList {
			if item.TotalAmount.CurrencyAmount != 0 {
				simplified = append(simplified, SimplifiedFinancialEvent{
					EventType:    "adjustment_item",
					PostedDate:   event.PostedDate,
					SellerSKU:    item.SellerSKU,
					ASIN:         item.ASIN,
					Amount:       item.TotalAmount.CurrencyAmount,
					CurrencyCode: item.TotalAmount.CurrencyCode,
					Description:  item.ProductDescription,
					Quantity:     0, // Convert string to int if needed
				})
			}
		}
	}

	// Process Debt Recovery Events
	for _, event := range financialEvents.DebtRecoveryEventList {
		if event.RecoveryAmount.CurrencyAmount != 0 {
			simplified = append(simplified, SimplifiedFinancialEvent{
				EventType:    "debt_recovery",
				Amount:       event.RecoveryAmount.CurrencyAmount,
				CurrencyCode: event.RecoveryAmount.CurrencyCode,
				Description:  event.DebtRecoveryType,
			})
		}
	}

	// Process Loan Servicing Events
	for _, event := range financialEvents.LoanServicingEventList {
		if event.LoanAmount.CurrencyAmount != 0 {
			simplified = append(simplified, SimplifiedFinancialEvent{
				EventType:    "loan_servicing",
				Amount:       event.LoanAmount.CurrencyAmount,
				CurrencyCode: event.LoanAmount.CurrencyCode,
				Description:  event.SourceBusinessEventType,
			})
		}
	}

	// Process Tax Withholding Events
	for _, event := range financialEvents.TaxWithholdingEventList {
		if event.WithheldAmount.CurrencyAmount != 0 {
			simplified = append(simplified, SimplifiedFinancialEvent{
				EventType:    "tax_withholding",
				PostedDate:   event.PostedDate,
				Amount:       event.WithheldAmount.CurrencyAmount,
				CurrencyCode: event.WithheldAmount.CurrencyCode,
				Description:  "Tax Withholding",
			})
		}
	}

	// Process Charge Refund Events
	for _, event := range financialEvents.ChargeRefundEventList {
		if event.RefundAmount.CurrencyAmount != 0 {
			simplified = append(simplified, SimplifiedFinancialEvent{
				EventType:    "charge_refund",
				PostedDate:   event.PostedDate,
				Amount:       event.RefundAmount.CurrencyAmount,
				CurrencyCode: event.RefundAmount.CurrencyCode,
				ReasonCode:   event.ReasonCode,
				ReferenceId:  event.ReferenceId,
			})
		}
	}

	return simplified
}

func (amazonTool *AmazonTool) mergeFinancialEvents(consolidated *FinancialEvents, current *FinancialEvents) {
	// Merge all 35+ event arrays from current page into consolidated
	consolidated.ShipmentEventList = append(consolidated.ShipmentEventList, current.ShipmentEventList...)
	consolidated.RefundEventList = append(consolidated.RefundEventList, current.RefundEventList...)
	consolidated.GuaranteeClaimEventList = append(consolidated.GuaranteeClaimEventList, current.GuaranteeClaimEventList...)
	consolidated.ChargebackEventList = append(consolidated.ChargebackEventList, current.ChargebackEventList...)
	consolidated.PayWithAmazonEventList = append(consolidated.PayWithAmazonEventList, current.PayWithAmazonEventList...)
	consolidated.ServiceProviderCreditEventList = append(consolidated.ServiceProviderCreditEventList, current.ServiceProviderCreditEventList...)
	consolidated.RetrochargeEventList = append(consolidated.RetrochargeEventList, current.RetrochargeEventList...)
	consolidated.RentalTransactionEventList = append(consolidated.RentalTransactionEventList, current.RentalTransactionEventList...)
	consolidated.ProductAdsPaymentEventList = append(consolidated.ProductAdsPaymentEventList, current.ProductAdsPaymentEventList...)
	consolidated.ServiceFeeEventList = append(consolidated.ServiceFeeEventList, current.ServiceFeeEventList...)
	consolidated.SellerDealPaymentEventList = append(consolidated.SellerDealPaymentEventList, current.SellerDealPaymentEventList...)
	consolidated.DebtRecoveryEventList = append(consolidated.DebtRecoveryEventList, current.DebtRecoveryEventList...)
	consolidated.LoanServicingEventList = append(consolidated.LoanServicingEventList, current.LoanServicingEventList...)
	consolidated.AdjustmentEventList = append(consolidated.AdjustmentEventList, current.AdjustmentEventList...)
	consolidated.SAFETReimbursementEventList = append(consolidated.SAFETReimbursementEventList, current.SAFETReimbursementEventList...)
	consolidated.SellerReviewEnrollmentPaymentEventList = append(consolidated.SellerReviewEnrollmentPaymentEventList, current.SellerReviewEnrollmentPaymentEventList...)
	consolidated.FBALiquidationEventList = append(consolidated.FBALiquidationEventList, current.FBALiquidationEventList...)
	consolidated.CouponPaymentEventList = append(consolidated.CouponPaymentEventList, current.CouponPaymentEventList...)
	consolidated.ImagingServicesFeeEventList = append(consolidated.ImagingServicesFeeEventList, current.ImagingServicesFeeEventList...)
	consolidated.NetworkComminglingTransactionEventList = append(consolidated.NetworkComminglingTransactionEventList, current.NetworkComminglingTransactionEventList...)
	consolidated.AffordabilityExpenseEventList = append(consolidated.AffordabilityExpenseEventList, current.AffordabilityExpenseEventList...)
	consolidated.AffordabilityExpenseReversalEventList = append(consolidated.AffordabilityExpenseReversalEventList, current.AffordabilityExpenseReversalEventList...)
	consolidated.RemovalShipmentEventList = append(consolidated.RemovalShipmentEventList, current.RemovalShipmentEventList...)
	consolidated.RemovalShipmentAdjustmentEventList = append(consolidated.RemovalShipmentAdjustmentEventList, current.RemovalShipmentAdjustmentEventList...)
	consolidated.TrialShipmentEventList = append(consolidated.TrialShipmentEventList, current.TrialShipmentEventList...)
	consolidated.TDSReimbursementEventList = append(consolidated.TDSReimbursementEventList, current.TDSReimbursementEventList...)
	consolidated.AdhocDisbursementEventList = append(consolidated.AdhocDisbursementEventList, current.AdhocDisbursementEventList...)
	consolidated.TaxWithholdingEventList = append(consolidated.TaxWithholdingEventList, current.TaxWithholdingEventList...)
	consolidated.ChargeRefundEventList = append(consolidated.ChargeRefundEventList, current.ChargeRefundEventList...)
	consolidated.FailedAdhocDisbursementEventList = append(consolidated.FailedAdhocDisbursementEventList, current.FailedAdhocDisbursementEventList...)
	consolidated.ValueAddedServiceChargeEventList = append(consolidated.ValueAddedServiceChargeEventList, current.ValueAddedServiceChargeEventList...)
	consolidated.CapacityReservationBillingEventList = append(consolidated.CapacityReservationBillingEventList, current.CapacityReservationBillingEventList...)
	consolidated.PaymentEventList = append(consolidated.PaymentEventList, current.PaymentEventList...)
	consolidated.CreateInboundShipmentPlanEventList = append(consolidated.CreateInboundShipmentPlanEventList, current.CreateInboundShipmentPlanEventList...)
	consolidated.ShippingLabelEventList = append(consolidated.ShippingLabelEventList, current.ShippingLabelEventList...)
}

func (amazonTool *AmazonTool) StopAutoRenew(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	if amazonTool.Scheduler == nil {
		err = errors.New("scheduler not defined")
		logs.Err(ctx, err, "")
		return nil, false, err
	}
	amazonTool.Scheduler.Unschedule(ctx, "", fmt.Sprint(amazonTool.Tool.Hooks.ARRT, "_", tenantId))
	toolResult = make(map[string]interface{})
	toolResult["stop_auto_renew_status"] = "success"
	amazonTool.AmazonAccount.TokenExpirationDateTime = ""
	persistStore = true
	return toolResult, persistStore, nil
}
