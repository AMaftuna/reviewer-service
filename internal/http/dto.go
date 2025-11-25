package httpx

// /team/add
type TeamAddReq struct {
	TeamName string `json:"team_name"`
	Members  []struct {
		UserID   string `json:"user_id"`
		Username string `json:"username"`
		IsActive bool   `json:"is_active"`
	} `json:"members"`
}

// /team/deactivate
type TeamDeactivateReq struct {
	TeamName string   `json:"team_name"`
	UserIDs  []string `json:"user_ids,omitempty"`
}

// /users/setIsActive
type SetIsActiveReq struct {
	UserID   string `json:"user_id"`
	IsActive bool   `json:"is_active"`
}

// /pullRequest/create
type PRCreateReq struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
}

// /pullRequest/merge
type PRMergeReq struct {
	PullRequestID string `json:"pull_request_id"`
}

// /pullRequest/reassign
type PRReassignReq struct {
	PullRequestID string `json:"pull_request_id"`
	OldUserID     string `json:"old_user_id"`
}
