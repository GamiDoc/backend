package design

type FAQEntry struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

var sectionFAQ = map[int][]FAQEntry{
	1: {
		{Question: "What belongs in the Context section?", Answer: "The domain, the target audience, and the concrete problem the gamified system should address."},
		{Question: "How specific should the audience be?", Answer: "Name a real group with shared traits, such as commuting students or warehouse staff, rather than a generic public."},
	},
	2: {
		{Question: "What is the Experience Timeline?", Answer: "A description of how a user meets and uses the system over time: onboarding, a typical session, and long-term evolution."},
		{Question: "How far ahead should the timeline reach?", Answer: "Cover at least the first contact, the steady state, and what keeps the experience alive after the novelty fades."},
	},
	3: {
		{Question: "What are personification and dynamics?", Answer: "The personas or player types you design for, and the social or competitive dynamics between them."},
		{Question: "Do I need formal player typologies?", Answer: "No. Informal personas grounded in your audience work; typologies such as HEXAD can help structure them."},
	},
	4: {
		{Question: "What is the Gameful Core?", Answer: "The central mechanics of the system: rules, goals, challenges, and reward structures."},
		{Question: "How many mechanics should I define?", Answer: "Start from the few mechanics that carry the core loop; supporting mechanics can be added once the loop is clear."},
	},
	5: {
		{Question: "What goes into Technology?", Answer: "Platforms, devices, integrations, data flows, and the technical constraints that shape the design."},
		{Question: "Should I fix the stack here?", Answer: "Name the constraints that are already fixed and mark open choices explicitly."},
	},
	6: {
		{Question: "What are Impacts and Benefits?", Answer: "The behavioural, learning, or business outcomes the system intends to produce."},
		{Question: "How do I phrase an impact?", Answer: "As an observable change with a direction, such as more weekly active-travel trips, rather than an abstract goal."},
	},
	7: {
		{Question: "What belongs in Evaluation and Feedback?", Answer: "How the system's effect will be measured, and how feedback loops reach the users."},
		{Question: "What makes a good evaluation plan?", Answer: "A measurable outcome, a comparison point, and a decision that depends on the result."},
	},
}

func SectionFAQ(number int) []FAQEntry {
	return sectionFAQ[number]
}
