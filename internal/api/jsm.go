package api

import (
	"context"
	"net/url"

	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
)

// Jira Service Management types and accessors.
//
// The official Rovo MCP server covers only JSM's Ops alerts and on-call schedules; the
// request/service-desk/queue surface that service teams actually work in is not exposed
// there at all.

const jsmBase = "/rest/servicedeskapi"

// ServiceDesk is a JSM service desk (the customer-facing view of a project).
type ServiceDesk struct {
	ID          ID     `json:"id,omitempty"`
	ProjectID   ID     `json:"projectId,omitempty"`
	ProjectName string `json:"projectName,omitempty"`
	ProjectKey  string `json:"projectKey,omitempty"`
}

// CustomerRequest is a request raised in a service desk.
type CustomerRequest struct {
	IssueID       ID     `json:"issueId,omitempty"`
	IssueKey      string `json:"issueKey,omitempty"`
	RequestTypeID ID     `json:"requestTypeId,omitempty"`
	ServiceDeskID ID     `json:"serviceDeskId,omitempty"`
	CreatedDate   struct {
		Jira     string `json:"jira,omitempty"`
		Friendly string `json:"friendly,omitempty"`
	} `json:"createdDate,omitempty"`
	Reporter      Ref `json:"reporter,omitempty"`
	CurrentStatus struct {
		Status         string `json:"status,omitempty"`
		StatusCategory string `json:"statusCategory,omitempty"`
	} `json:"currentStatus,omitempty"`
	RequestFieldValues []struct {
		FieldID string `json:"fieldId,omitempty"`
		Label   string `json:"label,omitempty"`
	} `json:"requestFieldValues,omitempty"`
}

// RequestType is a request type offered by a service desk.
type RequestType struct {
	ID            ID            `json:"id,omitempty"`
	Name          string        `json:"name,omitempty"`
	Description   string        `json:"description,omitempty"`
	HelpText      string        `json:"helpText,omitempty"`
	ServiceDeskID ID            `json:"serviceDeskId,omitempty"`
	GroupIDs      StringOrSlice `json:"groupIds,omitempty"`
	IssueTypeID   ID            `json:"issueTypeId,omitempty"`
}

// Organization is a JSM customer organization.
type Organization struct {
	ID   ID     `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// Queue is a service desk queue.
type Queue struct {
	ID     ID            `json:"id,omitempty"`
	Name   string        `json:"name,omitempty"`
	JQL    string        `json:"jql,omitempty"`
	Fields StringOrSlice `json:"fields,omitempty"`
}

// Customer is a JSM customer account.
type Customer struct {
	AccountID    string `json:"accountId,omitempty"`
	Name         string `json:"name,omitempty"`
	Key          string `json:"key,omitempty"`
	EmailAddress string `json:"emailAddress,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	Active       Bool   `json:"active,omitempty"`
	TimeZone     string `json:"timeZone,omitempty"`
}

// JSM collections use start/limit with an isLastPage terminator.

func (c *Client) ServiceDesks() *Resource[ServiceDesk] {
	return NewResource[ServiceDesk](c, catalog.ProductJSM, jsmBase+"/servicedesk", PageStartLimit)
}

func (c *Client) CustomerRequests() *Resource[CustomerRequest] {
	return NewResource[CustomerRequest](c, catalog.ProductJSM, jsmBase+"/request", PageStartLimit)
}

func (c *Client) Organizations() *Resource[Organization] {
	return NewResource[Organization](c, catalog.ProductJSM, jsmBase+"/organization", PageStartLimit)
}

// ServiceDeskQueues lists a service desk's queues.
func (c *Client) ServiceDeskQueues(ctx context.Context, serviceDeskID string, limit, max int) ([]Queue, error) {
	return listNested[Queue](ctx, c, catalog.ProductJSM,
		jsmBase+"/servicedesk/"+url.PathEscape(serviceDeskID)+"/queue", nil, limit, max)
}

// QueueIssues lists the issues sitting in a queue.
func (c *Client) QueueIssues(ctx context.Context, serviceDeskID, queueID string, limit, max int) ([]Issue, error) {
	return listNested[Issue](ctx, c, catalog.ProductJSM,
		jsmBase+"/servicedesk/"+url.PathEscape(serviceDeskID)+"/queue/"+url.PathEscape(queueID)+"/issue",
		nil, limit, max)
}

// ServiceDeskRequestTypes lists the request types a service desk offers.
func (c *Client) ServiceDeskRequestTypes(ctx context.Context, serviceDeskID string, limit, max int) ([]RequestType, error) {
	return listNested[RequestType](ctx, c, catalog.ProductJSM,
		jsmBase+"/servicedesk/"+url.PathEscape(serviceDeskID)+"/requesttype", nil, limit, max)
}

// ServiceDeskCustomers lists a service desk's customers.
func (c *Client) ServiceDeskCustomers(ctx context.Context, serviceDeskID string, limit, max int) ([]Customer, error) {
	return listNested[Customer](ctx, c, catalog.ProductJSM,
		jsmBase+"/servicedesk/"+url.PathEscape(serviceDeskID)+"/customer", nil, limit, max)
}
