package protocol

type Collection struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Items   int    `json:"items"`
	Updated string `json:"updated"`
}

type CollectionItem struct {
	ID         string `json:"id"`
	Collection string `json:"collection"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Text       string `json:"text,omitempty"`
	Path       string `json:"path,omitempty"`
	URL        string `json:"url,omitempty"`
	Image      string `json:"image,omitempty"`
	Source     string `json:"source,omitempty"`
	Saved      string `json:"saved"`
	Checked    string `json:"checked,omitempty"`
	Changed    bool   `json:"changed"`
	Truncated  bool   `json:"truncated,omitempty"`
}

type CollectionsListParams struct{}
type CollectionsListResult struct {
	Collections []Collection `json:"collections"`
}
type CollectionSaveParams struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
}
type CollectionAddParams struct {
	Collection string `json:"collection,omitempty"`
	Kind       string `json:"kind"`
	Title      string `json:"title,omitempty"`
	Text       string `json:"text,omitempty"`
	Path       string `json:"path,omitempty"`
	URL        string `json:"url,omitempty"`
	Image      string `json:"image,omitempty"`
	Source     string `json:"source,omitempty"`
}
type CollectionItemsParams struct {
	Collection string `json:"collection"`
	ID         string `json:"id,omitempty"`
	Cursor     string `json:"cursor,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}
type CollectionItemsResult struct {
	Items []CollectionItem `json:"items,omitempty"`
	Item  *CollectionItem  `json:"item,omitempty"`
	Next  string           `json:"next,omitempty"`
	Total int              `json:"total"`
}
type CollectionRemoveParams struct {
	Collection string `json:"collection"`
	ID         string `json:"id"`
}
type CollectionCheckParams struct {
	Collection string `json:"collection"`
	ID         string `json:"id"`
}
type CollectionChatParams struct {
	Collection string `json:"collection"`
	Question   string `json:"question,omitempty"`
}
