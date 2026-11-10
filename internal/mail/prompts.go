package mail

// Starter prompts mirror web/src/connect.js (starterPrompts) for email copy.
var starterPrompts = []string{
	"Create a new canvasbin and put a game of tic tac toe on it.",
	"Create a canvasbin with the latest agentic news, create it in a newsroom style and make it a single page application with navigation and in depth articles.",
	"Make a newspaper-style front page of today's top stories and publish it to Canvasbin.",
	"Turn this spreadsheet into a dashboard with charts and give me a Canvasbin link and a PDF.",
	"Draw our system architecture as diagrams and publish it as a Canvasbin page.",
}

// useCaseLinks are short pointers to the marketing gallery (base is prepended).
var useCaseLinks = []emailButton{
	{Label: "News apps & games", URL: "/use-cases/news-apps-games"},
	{Label: "Business dashboards", URL: "/use-cases/business"},
	{Label: "Family & shared notes", URL: "/use-cases/family"},
	{Label: "All use cases", URL: "/use-cases"},
}

func absButtons(base string, btns []emailButton) []emailButton {
	out := make([]emailButton, len(btns))
	for i, b := range btns {
		out[i] = emailButton{Label: b.Label, URL: base + b.URL}
	}
	return out
}
