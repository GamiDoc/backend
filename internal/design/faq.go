package design

type FAQEntry struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

var sectionFAQ = map[int][]FAQEntry{
	1: {
		{Question: "What belongs in the Context section?", Answer: "The domain and aim of your software, a profile of your users and the behaviour you want to encourage, and the constraints your users or project may face."},
		{Question: "How specific should the user profile be?", Answer: "Name a real group with shared traits, such as commuting students or warehouse staff, rather than a generic public."},
	},
	2: {
		{Question: "What is the Experience Timeline?", Answer: "How the experience evolves over time: how users are onboarded, how often the system changes, what keeps them coming back, and where they might drop out."},
		{Question: "What is a retention anchor?", Answer: "A mechanism that ties users emotionally or practically to the system, such as a team that relies on them, a daily streak, or a collection they built."},
	},
	3: {
		{Question: "What are user types?", Answer: "Groups of users with the same primary motive, such as the HEXAD types (Philanthropist, Socialiser, Free Spirit, Achiever, Player, Disruptor)."},
		{Question: "What is the difference between player-driven and game-driven personalization?", Answer: "Player-driven personalization is what users choose themselves; game-driven systems adapt automatically to the current session; data-driven systems use accumulated data across sessions or players."},
	},
	4: {
		{Question: "What is the Gameful Core?", Answer: "The actual game: the rules, the loops that repeat, the game elements and mechanics that drive them, and how players move forward over time."},
		{Question: "What is the difference between the core loop and the long loop?", Answer: "The core loop is the basic action repeated every few seconds or minutes; the long loop gives it purpose over hours or the whole game, usually through progression and rewards."},
	},
	5: {
		{Question: "What is a feedback component?", Answer: "One piece of feedback the system gives: when it appears, what it communicates, its type, timing and modality, and how it is presented and implemented."},
		{Question: "Why define feedback before evaluation?", Answer: "Feedback is how players learn whether they are progressing; evaluation then checks whether the system as a whole reaches its goals."},
	},
	6: {
		{Question: "What goes into Technical requirements?", Answer: "The platforms, engine and technology stack, performance budgets, data and online features, integrations, content pipeline, and technical risks."},
		{Question: "Should I fix the stack here?", Answer: "Name the constraints that are already fixed and mark the open choices explicitly."},
	},
}

func SectionFAQ(number int) []FAQEntry {
	return sectionFAQ[number]
}
