package ipupdater

// ClientError - an error that's the client's fault (probably bad domain or key)
type ClientError struct {
	status string
}

func (e ClientError) Error() string {
	return "client error: " + e.status
}

// ServerError - a server-side error, usually temporary
type ServerError struct {
	status string
}

func (e ServerError) Error() string {
	return "server error: " + e.status
}
