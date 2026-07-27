package api

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
)

// Confluence Cloud v2 types and accessors, plus the v1 endpoints v2 has no equivalent for
// (most importantly CQL search).

const (
	confluenceBase   = "/wiki/api/v2"
	confluenceV1Base = "/wiki/rest/api"
)

// Body is Confluence's multi-representation content body. A page can be requested as
// storage (XHTML), atlas_doc_format (ADF) or view (rendered HTML), and the response carries
// whichever representations were asked for.
type Body struct {
	Storage        *BodyValue `json:"storage,omitempty"`
	AtlasDocFormat *BodyValue `json:"atlas_doc_format,omitempty"`
	View           *BodyValue `json:"view,omitempty"`
}

// BodyValue is one representation of a body.
type BodyValue struct {
	Representation string `json:"representation,omitempty"`
	Value          string `json:"value,omitempty"`
}

// Page is a Confluence page.
type Page struct {
	ID        ID              `json:"id,omitempty"`
	Status    string          `json:"status,omitempty"`
	Title     string          `json:"title,omitempty"`
	SpaceID   ID              `json:"spaceId,omitempty"`
	ParentID  ID              `json:"parentId,omitempty"`
	AuthorID  string          `json:"authorId,omitempty"`
	OwnerID   string          `json:"ownerId,omitempty"`
	CreatedAt string          `json:"createdAt,omitempty"`
	Version   *ContentVersion `json:"version,omitempty"`
	Body      *Body           `json:"body,omitempty"`
	Links     struct {
		WebUI string `json:"webui,omitempty"`
		Base  string `json:"base,omitempty"`
	} `json:"_links,omitempty"`
}

// ContentVersion is the version stamp every Confluence content update must supply.
type ContentVersion struct {
	Number    Int    `json:"number,omitempty"`
	Message   string `json:"message,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	AuthorID  string `json:"authorId,omitempty"`
	MinorEdit Bool   `json:"minorEdit,omitempty"`
}

// Space is a Confluence space.
type Space struct {
	ID          ID     `json:"id,omitempty"`
	Key         string `json:"key,omitempty"`
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"`
	Status      string `json:"status,omitempty"`
	HomepageID  ID     `json:"homepageId,omitempty"`
	Description struct {
		Plain *BodyValue `json:"plain,omitempty"`
	} `json:"description,omitempty"`
	Links struct {
		WebUI string `json:"webui,omitempty"`
	} `json:"_links,omitempty"`
}

// BlogPost is a Confluence blog post.
type BlogPost struct {
	ID        ID              `json:"id,omitempty"`
	Status    string          `json:"status,omitempty"`
	Title     string          `json:"title,omitempty"`
	SpaceID   ID              `json:"spaceId,omitempty"`
	AuthorID  string          `json:"authorId,omitempty"`
	CreatedAt string          `json:"createdAt,omitempty"`
	Version   *ContentVersion `json:"version,omitempty"`
	Body      *Body           `json:"body,omitempty"`
}

// ConfluenceComment is a footer or inline comment on Confluence content.
type ConfluenceComment struct {
	ID         ID              `json:"id,omitempty"`
	Status     string          `json:"status,omitempty"`
	Title      string          `json:"title,omitempty"`
	PageID     ID              `json:"pageId,omitempty"`
	BlogPostID ID              `json:"blogPostId,omitempty"`
	ParentID   ID              `json:"parentCommentId,omitempty"`
	Version    *ContentVersion `json:"version,omitempty"`
	Body       *Body           `json:"body,omitempty"`
}

// Label is a Confluence label.
type Label struct {
	ID     ID     `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	Prefix string `json:"prefix,omitempty"`
}

// ConfluenceAttachment is a file attached to Confluence content.
type ConfluenceAttachment struct {
	ID                   ID              `json:"id,omitempty"`
	Status               string          `json:"status,omitempty"`
	Title                string          `json:"title,omitempty"`
	MediaType            string          `json:"mediaType,omitempty"`
	MediaTypeDescription string          `json:"mediaTypeDescription,omitempty"`
	FileSize             Int             `json:"fileSize,omitempty"`
	Comment              string          `json:"comment,omitempty"`
	PageID               ID              `json:"pageId,omitempty"`
	Version              *ContentVersion `json:"version,omitempty"`
	DownloadLink         string          `json:"downloadLink,omitempty"`
}

// Whiteboard is a Confluence whiteboard.
type Whiteboard struct {
	ID        ID     `json:"id,omitempty"`
	Title     string `json:"title,omitempty"`
	SpaceID   ID     `json:"spaceId,omitempty"`
	ParentID  ID     `json:"parentId,omitempty"`
	AuthorID  string `json:"authorId,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// Database is a Confluence database.
type Database struct {
	ID        ID     `json:"id,omitempty"`
	Title     string `json:"title,omitempty"`
	SpaceID   ID     `json:"spaceId,omitempty"`
	ParentID  ID     `json:"parentId,omitempty"`
	AuthorID  string `json:"authorId,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// Folder is a Confluence folder.
type Folder struct {
	ID        ID     `json:"id,omitempty"`
	Title     string `json:"title,omitempty"`
	SpaceID   ID     `json:"spaceId,omitempty"`
	ParentID  ID     `json:"parentId,omitempty"`
	AuthorID  string `json:"authorId,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// CustomContent is a Confluence custom content item.
type CustomContent struct {
	ID      ID              `json:"id,omitempty"`
	Type    string          `json:"type,omitempty"`
	Status  string          `json:"status,omitempty"`
	Title   string          `json:"title,omitempty"`
	SpaceID ID              `json:"spaceId,omitempty"`
	PageID  ID              `json:"pageId,omitempty"`
	Version *ContentVersion `json:"version,omitempty"`
	Body    *Body           `json:"body,omitempty"`
}

// Confluence v2 collections are all cursor-paginated.

func (c *Client) Pages() *Resource[Page] {
	return NewResource[Page](c, catalog.ProductConfluence, confluenceBase+"/pages", PageCursor)
}

func (c *Client) Spaces() *Resource[Space] {
	return NewResource[Space](c, catalog.ProductConfluence, confluenceBase+"/spaces", PageCursor)
}

func (c *Client) BlogPosts() *Resource[BlogPost] {
	return NewResource[BlogPost](c, catalog.ProductConfluence, confluenceBase+"/blogposts", PageCursor)
}

func (c *Client) ConfluenceComments() *Resource[ConfluenceComment] {
	return NewResource[ConfluenceComment](c, catalog.ProductConfluence, confluenceBase+"/footer-comments", PageCursor)
}

func (c *Client) ConfluenceAttachments() *Resource[ConfluenceAttachment] {
	return NewResource[ConfluenceAttachment](c, catalog.ProductConfluence, confluenceBase+"/attachments", PageCursor)
}

func (c *Client) Whiteboards() *Resource[Whiteboard] {
	return NewResource[Whiteboard](c, catalog.ProductConfluence, confluenceBase+"/whiteboards", PageCursor)
}

func (c *Client) Databases() *Resource[Database] {
	return NewResource[Database](c, catalog.ProductConfluence, confluenceBase+"/databases", PageCursor)
}

func (c *Client) Folders() *Resource[Folder] {
	return NewResource[Folder](c, catalog.ProductConfluence, confluenceBase+"/folders", PageCursor)
}

func (c *Client) CustomContent() *Resource[CustomContent] {
	return NewResource[CustomContent](c, catalog.ProductConfluence, confluenceBase+"/custom-content", PageCursor)
}

func (c *Client) ConfluenceLabels() *Resource[Label] {
	return NewResource[Label](c, catalog.ProductConfluence, confluenceBase+"/labels", PageCursor)
}

// CQLResult is a Confluence search response.
type CQLResult struct {
	Results        []CQLMatch `json:"results"`
	Start          int        `json:"start"`
	Limit          int        `json:"limit"`
	Size           int        `json:"size"`
	TotalSize      int        `json:"totalSize"`
	CQLQuery       string     `json:"cqlQuery,omitempty"`
	SearchDuration int        `json:"searchDuration,omitempty"`
	Links          struct {
		Next string `json:"next,omitempty"`
		Base string `json:"base,omitempty"`
	} `json:"_links,omitempty"`
}

// CQLMatch is one search hit.
type CQLMatch struct {
	Content struct {
		ID     ID     `json:"id,omitempty"`
		Type   string `json:"type,omitempty"`
		Status string `json:"status,omitempty"`
		Title  string `json:"title,omitempty"`
	} `json:"content,omitempty"`
	Title                 string          `json:"title,omitempty"`
	Excerpt               string          `json:"excerpt,omitempty"`
	URL                   string          `json:"url,omitempty"`
	EntityType            string          `json:"entityType,omitempty"`
	LastModified          string          `json:"lastModified,omitempty"`
	FriendlyLastModified  string          `json:"friendlyLastModified,omitempty"`
	ResultGlobalContainer json.RawMessage `json:"resultGlobalContainer,omitempty"`
}

// SearchCQL runs a Confluence Query Language search.
//
// This lives on the v1 API because v2 has no search endpoint at all — the single strongest
// reason the v1 document is still shipped in the catalog (DECISIONS.md #2).
func (c *Client) SearchCQL(ctx context.Context, cql, cqlContext string, limit, start int, expand string) (*CQLResult, error) {
	q := url.Values{}
	q.Set("cql", cql)
	if cqlContext != "" {
		q.Set("cqlcontext", cqlContext)
	}
	if limit > 0 {
		q.Set("limit", itoa(limit))
	}
	if start > 0 {
		q.Set("start", itoa(start))
	}
	if expand != "" {
		q.Set("expand", expand)
	}

	var out CQLResult
	if err := c.GetJSON(ctx, catalog.ProductConfluenceV1, confluenceV1Base+"/search", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SearchCQLAll walks every page of a CQL search up to max (0 = no ceiling).
func (c *Client) SearchCQLAll(ctx context.Context, cql, cqlContext string, limit, max int, expand string) ([]CQLMatch, error) {
	var out []CQLMatch
	start := 0
	pageSize := limit
	if pageSize <= 0 {
		pageSize = 50
	}
	for {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		page, err := c.SearchCQL(ctx, cql, cqlContext, pageSize, start, expand)
		if err != nil {
			return out, err
		}
		out = append(out, page.Results...)

		if max > 0 && len(out) >= max {
			return out[:max], nil
		}
		// A short page means the end; `_links.next` confirms it when present.
		if len(page.Results) == 0 || page.Links.Next == "" || len(page.Results) < pageSize {
			return out, nil
		}
		start += len(page.Results)
	}
}

// PageBody fetches a page with a specific body representation, which the plain get does not
// return by default (Confluence omits bodies unless asked).
func (c *Client) PageBody(ctx context.Context, id, representation string) (*Page, error) {
	if representation == "" {
		representation = "storage"
	}
	var p Page
	err := c.GetJSON(ctx, catalog.ProductConfluence,
		confluenceBase+"/pages/"+url.PathEscape(id),
		url.Values{"body-format": {representation}}, &p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
