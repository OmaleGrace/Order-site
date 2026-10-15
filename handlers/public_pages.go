package handlers

import (
	"html/template"
	"log"
	"net/http"

	"Order-site/brand"
)

type PublicPageSection struct {
	Heading    string
	Paragraphs []string
	Items      []string
}

type PublicPageData struct {
	Title        string
	Intro        string
	Sections     []PublicPageSection
	ContactEmail string
}

func (h *Handlers) PublicPage(w http.ResponseWriter, r *http.Request) {
	const contactEmail = "omalegrace209@gmail.com"

	pages := map[string]PublicPageData{
		"/about": {
			Title: "About Us",
			Intro: "Grace's Kitchen helps customers discover meals from participating food vendors and place orders online.",
			Sections: []PublicPageSection{
				{
					Heading: "What we do",
					Paragraphs: []string{
						"Our platform brings food vendors and customers together in one place. Visitors can explore participating shops, discover available meals, and create an account to use ordering features.",
						"Registered customers can place orders, make payments through the available payment process, and check their order information.",
					},
				},
				{
					Heading: "Our vendors",
					Paragraphs: []string{
						"Menus, prices, availability, and fulfilment may vary by vendor. Customers should review the details shown for a shop and their order before completing checkout.",
					},
				},
				{
					Heading: "Get in touch",
					Paragraphs: []string{
						"If you have a question about the platform, please contact us using the email address on our Contact page.",
					},
				},
			},
			ContactEmail: contactEmail,
		},

		"/contact": {
			Title: "Contact Us",
			Intro: "Have a question about Grace's Kitchen? You can reach us using the contact details below.",
			Sections: []PublicPageSection{
				{
					Heading: "Email",
					Paragraphs: []string{
						"For general questions, website feedback, or help with the platform, email us at the address below.",
					},
				},
				{
					Heading: "Order enquiries",
					Paragraphs: []string{
						"If your enquiry concerns a particular order, include its reference if available. Do not include your password or full payment-card details in your message.",
					},
				},
			},
			ContactEmail: contactEmail,
		},

		"/privacy": {
			Title: "Privacy Policy",
			Intro: "This policy explains how Grace's Kitchen handles information when you visit or use the website.",
			Sections: []PublicPageSection{
				{
					Heading: "Information we may collect",
					Paragraphs: []string{
						"Depending on how you use the website, information may include your name, email address, account credentials, phone number, delivery details, order history, and information you submit through account or ordering forms.",
						"The website may also process session information and technical data needed to operate, secure, and troubleshoot the service.",
					},
				},
				{
					Heading: "How information is used",
					Paragraphs: []string{
						"We use relevant information to create and manage accounts, authenticate users, process orders, communicate about transactions, support customers, and protect the website against misuse.",
						"Account passwords should be stored in protected, non-plaintext form. Never send your password to us by email.",
					},
				},
				{
					Heading: "Payments and service providers",
					Paragraphs: []string{
						"When you make an online payment, payment processing is handled through Paystack. Paystack may process payment information under its own privacy terms.",
						"The website may use Brevo to send service-related emails. If you choose Google sign-in, Google may process information needed to authenticate you.",
						"Information may also be shared with the relevant vendor when necessary to fulfil an order, such as providing the order details and delivery information required for fulfilment.",
					},
				},
				{
					Heading: "Cookies and sessions",
					Paragraphs: []string{
						"The website uses session-related information to support login and other essential features. Blocking cookies or session storage may prevent some features from working correctly.",
					},
				},
				{
					Heading: "Storage, security, and retention",
					Paragraphs: []string{
						"We use reasonable measures intended to protect information handled by the service. No internet-based system can be guaranteed completely secure.",
						"Information may be retained for as long as needed to operate the service, maintain relevant records, resolve disputes, and meet applicable obligations. Actual retention depends on the type of information and its purpose.",
					},
				},
				{
					Heading: "Your choices and questions",
					Paragraphs: []string{
						"You can contact us to ask about personal information associated with your account or to request an appropriate correction or deletion. Some information may need to be retained where required for legitimate operational or legal reasons.",
						"We may update this policy as the website changes. Updates will be published on this page.",
					},
				},
			},
			ContactEmail: contactEmail,
		},

		"/terms": {
			Title: "Terms of Service",
			Intro: "These terms describe the basic conditions for using Grace's Kitchen. By using the website, you agree to use it responsibly and comply with applicable requirements.",
			Sections: []PublicPageSection{
				{
					Heading: "Using the website",
					Paragraphs: []string{
						"You must provide accurate information when creating an account or placing an order. You are responsible for keeping your login credentials private and for activity carried out through your account.",
						"Do not misuse the website, interfere with its operation, attempt unauthorised access, or submit fraudulent or misleading information.",
					},
				},
				{
					Heading: "Menus, prices, and availability",
					Paragraphs: []string{
						"Food items, descriptions, prices, and availability are provided through participating vendors and may change. Please review your order details before submitting it.",
						"An item being displayed on the website does not guarantee that it will always be available.",
					},
				},
				{
					Heading: "Orders and fulfilment",
					Paragraphs: []string{
						"Orders are subject to the details displayed during checkout and confirmation. Preparation, fulfilment, and delivery depend on the relevant vendor and the options available for the order.",
						"If there is a problem with an order, contact us with the order reference where possible so the issue can be reviewed.",
					},
				},
				{
					Heading: "Payments, cancellations, and refunds",
					Paragraphs: []string{
						"Payments are processed using the payment options presented on the website. Payment transactions may also be subject to the payment provider's terms.",
						"Cancellation and refund requests depend on the circumstances of the order and applicable requirements. Contact us about a specific issue rather than assuming every order can be cancelled or refunded automatically.",
					},
				},
				{
					Heading: "Website availability and changes",
					Paragraphs: []string{
						"We may update, suspend, or change website features to maintain or improve the service. We aim to keep the website useful and available, but uninterrupted operation cannot be guaranteed.",
					},
				},
				{
					Heading: "Contact and updates",
					Paragraphs: []string{
						"If you have a question about these terms, contact us using the email address on this page. We may revise these terms as the service develops; the latest version will be published here.",
						"These general terms do not remove any rights that cannot lawfully be excluded under applicable law.",
					},
				},
			},
			ContactEmail: contactEmail,
		},
	}

	page, ok := pages[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}

	tmpl, err := template.New("public-page.html").
		Funcs(brand.Funcs()).
		ParseFiles("templates/public-page.html")
	if err != nil {
		log.Printf("public page template error: %v", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("public page render error: %v", err)
	}
}
