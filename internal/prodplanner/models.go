package prodplanner

type Ticket struct {
	ID              int         `json:"id"`
	FormattedNumber string      `json:"formatted_number"`
	Type            string      `json:"type"`
	Title           string      `json:"title"`
	Description     string      `json:"description"`
	Priority        string      `json:"priority"`
	Size            string      `json:"size"`
	AssignedTo      *int        `json:"assigned_to"`
	BoardColumn     BoardColumn `json:"board_column"`
	Project         TicketProject `json:"project"`
}

type BoardColumn struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type TicketProject struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	TicketPrefix string `json:"ticket_prefix"`
}

type Project struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	TicketPrefix string `json:"ticket_prefix"`
}

type ListTicketsOptions struct {
	AssignedTo int
	ProjectID  int
	ColumnID   int
	Status     string // "active" or "completed"
}
