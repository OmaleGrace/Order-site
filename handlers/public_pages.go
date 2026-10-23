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
			Intro: brand.Name() + " helps customers discover meals from participating food vendors and place orders online.",
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

		"/guides": {
			Title: "Nigerian Food and Ordering Guides",
			Intro: "Practical, original guides to exploring Nigerian meals, choosing pickup or delivery, and ordering confidently from participating food shops.",
			Sections: []PublicPageSection{
				{
					Heading: "Explore Nigerian meals by occasion",
					Paragraphs: []string{
						"Rice dishes such as jollof rice and fried rice are common choices for celebrations and group meals. If you are ordering for several people, check the portion size and whether sides or protein are included before comparing prices.",
						"Soups such as egusi, okra, efo riro, and banga are often paired with swallows including eba, pounded yam, amala, or fufu. Since recipes and portion sizes vary between kitchens, read each shop's description and ask the vendor about ingredients or spice levels when those details matter to you.",
					},
				},
				{
					Heading: "How to compare food listings",
					Paragraphs: []string{
						"Compare the full order rather than the headline price alone. Check the listed portion, included sides, preparation notes, pickup or delivery availability, delivery fee, and estimated fulfilment information shown during checkout.",
						"Menu photographs can help you understand a dish, but they may be illustrative and presentation can vary. Use the item name, description, and vendor details together when deciding what to order.",
					},
				},
				{
					Heading: "Food allergies and dietary needs",
					Paragraphs: []string{
						"If you have a food allergy or a dietary restriction, contact the vendor before placing an order. Recipes, cooking oils, seasonings, and preparation areas differ by kitchen, and an online menu may not list every ingredient or cross-contact risk.",
						"Do not rely on a photograph alone to determine whether a dish is suitable for you. Ask specific questions and only order when you are comfortable with the information the vendor provides.",
					},
				},
			},
		},
		"/guides/nigerian-rice-dishes": {
			Title: "A Guide to Nigerian Rice Dishes",
			Intro: "Jollof rice, fried rice, and white rice with stew offer different flavours and pairings. Use this guide to compare them when browsing a food menu.",
			Sections: []PublicPageSection{
				{
					Heading: "Jollof rice",
					Paragraphs: []string{
						"Jollof rice is cooked with a seasoned tomato-and-pepper base, though the exact recipe varies by cook and region. It is often served with chicken, beef, fish, plantain, or salad. When ordering, check which sides are included instead of assuming every listing comes with the same extras.",
					},
				},
				{
					Heading: "Fried rice",
					Paragraphs: []string{
						"Nigerian-style fried rice commonly combines rice with vegetables and seasoning, and it is often served alongside grilled or fried protein. Ask the vendor about ingredients if you need to avoid a particular vegetable, seasoning, or cooking oil.",
					},
				},
				{
					Heading: "White rice and stew",
					Paragraphs: []string{
						"White rice with stew is a flexible meal because the stew and protein can vary between kitchens. Check the item description for the stew style, protein choice, and portion information before checkout.",
					},
				},
				{
					Heading: "Choosing the right listing",
					Paragraphs: []string{
						"Compare portion size, included sides, preparation notes, price, and pickup or delivery options. If an item description leaves out information that affects your decision, ask the vendor before paying.",
					},
				},
				{
					Heading: "Party rice and everyday rice",
					Paragraphs: []string{
						"Jollof rice served at large gatherings is often cooked in big pots over open heat, which can give it a smoky flavour that a smaller kitchen pot does not produce. Everyday jollof made for a single order is usually cooked on a stove and tastes lighter. Neither is better; they are simply different, so it helps to know which style a vendor is describing.",
						"If you are ordering for an event, ask the vendor how far ahead they need notice, how many portions their pots can handle, and whether they provide serving containers. Large orders are easier to get right when the details are agreed before payment.",
					},
				},
				{
					Heading: "Typical sides and how they change the price",
					Paragraphs: []string{
						"Fried plantain, moi moi, coleslaw, salad, and a choice of chicken, beef, turkey, or fish are common additions. Some vendors list rice as a base price and charge for each extra, while others price a full plate. Comparing a base price with a full-plate price is a common reason two listings look very different in cost.",
						"A simple way to compare is to write down what you actually want on the plate, then add up the listed price for exactly that combination from each vendor. The cheapest headline price is not always the cheapest finished plate.",
					},
				},
				{
					Heading: "Heat, spice and personal taste",
					Paragraphs: []string{
						"Pepper levels vary a great deal between cooks. If you prefer mild food, or you are ordering for children or older relatives, say so in your order notes or contact the vendor before paying. Many cooks will adjust the pepper when they know about it in advance.",
					},
				},
			},
		},
		"/guides/ordering-food-safely": {
			Title: "How to Order Food Online with Confidence",
			Intro: "A practical checklist for reviewing a food listing, checking order details, and keeping a clear record of your online order.",
			Sections: []PublicPageSection{
				{
					Heading: "Review the shop and menu",
					Paragraphs: []string{
						"Read the shop description and item details, check the listed price and available fulfilment methods, and review any verification information shown on the shop page. A verification badge should only be treated as evidence of the specific checks the platform says it performs, not as a guarantee of every aspect of a transaction.",
					},
				},
				{
					Heading: "Confirm your order before payment",
					Paragraphs: []string{
						"Before submitting an order, check quantities, item choices, delivery or pickup method, delivery address, contact phone number, fees, and the final total. Keep your order reference and confirmation message in case you need to ask a question later.",
					},
				},
				{
					Heading: "Protect your account",
					Paragraphs: []string{
						"Use a unique password, sign out on shared devices, and never send your password or full payment-card details in a support message. Only complete payment through the payment flow presented by the website.",
					},
				},
				{
					Heading: "If something goes wrong",
					Paragraphs: []string{
						"Contact the platform or vendor using the published contact route and include your order reference. Explain the issue clearly and keep relevant receipts or order confirmations. Refund and cancellation outcomes depend on the circumstances and applicable terms.",
					},
				},
				{
					Heading: "Spotting a well-described listing",
					Paragraphs: []string{
						"A helpful listing tells you what the dish is, roughly how much food you receive, what is included, and whether anything is optional. Listings that only show a photograph and a price leave more room for misunderstanding. If a description is thin, a short message to the vendor before you order is a sensible step.",
						"Prices should be shown in naira and the total at checkout should match what you expected. If the total includes a delivery fee, it should be shown before you pay, not after.",
					},
				},
				{
					Heading: "Paying safely",
					Paragraphs: []string{
						"Pay only through the checkout flow on the website. Card details are entered on the payment provider's page, not typed into a chat or sent in a message. Be cautious of anyone who asks you to transfer money to a personal account, or who asks for a card number, PIN, or one-time code by phone or message.",
						"After you pay, keep the confirmation email or the order number. If the page says your payment is processing, wait for the confirmation before paying a second time, to avoid being charged twice.",
					},
				},
				{
					Heading: "Receiving your order",
					Paragraphs: []string{
						"When your food arrives or you collect it, check that the items match the order. If something is missing or wrong, contact the vendor promptly with your order number and describe the problem clearly. A calm, specific message with the order number gets a faster and more useful reply than a general complaint.",
					},
				},
			},
		},
		"/guides/pickup-or-delivery": {
			Title: "Food Pickup or Delivery: How to Choose",
			Intro: "Pickup and delivery can both work well. The best choice depends on the shop's options, distance, timing, and the details shown at checkout.",
			Sections: []PublicPageSection{
				{
					Heading: "When pickup may suit you",
					Paragraphs: []string{
						"Pickup can be useful if you are already nearby or prefer to collect the order yourself. Before travelling, check the shop's pickup address and any instructions, and wait for the order status or vendor confirmation that indicates it is ready.",
					},
				},
				{
					Heading: "When delivery may suit you",
					Paragraphs: []string{
						"Delivery can save a trip when the shop offers service to your area. Confirm that your address and phone number are correct, and review the delivery fee and any available timing information before paying.",
					},
				},
				{
					Heading: "Compare the total cost and instructions",
					Paragraphs: []string{
						"The item price is only one part of the decision. Check the final checkout total, fulfilment method, vendor instructions, and the contact details available if you need help. If delivery coverage or timing is unclear, ask the shop before submitting the order.",
					},
				},
				{
					Heading: "Planning around timing",
					Paragraphs: []string{
						"Food that is made to order takes time to prepare before it is collected or sent out. At busy hours such as lunchtime and early evening, preparation may take longer, and delivery may depend on traffic and the distance from the vendor. If you have a firm deadline, order earlier than you think you need to and check the status shown on your orders page.",
						"For pickup, arrive when your order is marked ready rather than when you placed it. Food collected promptly is usually better than food left waiting.",
					},
				},
				{
					Heading: "Getting the delivery details right",
					Paragraphs: []string{
						"A complete address includes the street, a nearby landmark if the street is hard to find, your local government area, and your state. Add a phone number that you will answer, because delivery riders often need to call. Most failed deliveries happen because the address is incomplete or the phone is switched off.",
						"If you will not be home, tell the vendor who can receive the order. Do not leave food outside for long, especially in hot weather.",
					},
				},
				{
					Heading: "Costs to keep in mind",
					Paragraphs: []string{
						"Delivery fees are set by the vendor and may depend on distance. Pickup has no delivery fee, but you may spend on transport. For a single meal, pickup can be cheaper if the shop is close; for a group order, delivery often costs less per person than everyone travelling separately.",
					},
				},
			},
		},
		"/contact": {
			Title: "Contact Us",
			Intro: "Have a question about " + brand.Name() + "? You can reach us using the contact details below.",
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
			Intro: "This policy explains how " + brand.Name() + " handles information when you visit or use the website.",
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
						"Account passwords are stored in hashed form, not as plain text. Never send your password to us by email.",
					},
				},
				{
					Heading: "Vendor applications",
					Paragraphs: []string{
						"If you apply to sell on the platform, we collect your business name, your full name, phone number, shop area and address, the kind of ID you will show us, an optional business link, and any notes you add.",
						"We check your identity by speaking to you and viewing your ID in person or on a live video call. We do not ask you to submit your ID number or a photo of your ID through the website, and we do not store them. We record which kind of ID was checked, how it was checked, and our decision.",
					},
				},
				{
					Heading: "Payments and service providers",
					Paragraphs: []string{
						"When you make an online payment, payment processing is handled through Paystack. Paystack may process payment information under its own privacy terms.",
						"The website may use Brevo to send service-related emails, Termii to send phone verification codes by SMS, and Cloudinary to store profile pictures you upload. If you choose Google sign-in, Google may process information needed to authenticate you.",
						"Information may also be shared with the relevant vendor when necessary to fulfil an order, such as providing the order details and delivery information required for fulfilment.",
					},
				},
				{
					Heading: "Advertising and third-party cookies",
					Paragraphs: []string{
						"If Google AdSense or another advertising service is enabled on this website, that service and its partners may use cookies, web beacons, IP addresses, or similar technologies to deliver, measure, and personalise ads where permitted. Third-party vendors, including Google, may use information from visits to this and other websites to provide advertising services.",
						"Visitors can manage ad personalisation through Google's ad settings and can learn how Google uses information from partner sites through Google's published privacy information. The availability of controls depends on the visitor's location, device, and account settings.",
						"We will update this policy if our advertising practices materially change. Where applicable law requires consent for non-essential cookies or personalised advertising, the appropriate consent choices must be configured before those technologies are used.",
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
			Intro: "These terms describe the basic conditions for using " + brand.Name() + ". By using the website, you agree to use it responsibly and comply with applicable requirements.",
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
					Heading: "Terms for vendors",
					Paragraphs: []string{
						"To sell on the platform you must apply, pass our identity check, and pay the one-time setup fee shown on the Sell with us page. There is no monthly fee. We open a shop only after approval and payment.",
						"On every order we keep a commission of the food total shown on the Sell with us page. Delivery fees are not part of the commission. Customer payments are received by us, and we pay the vendor's share to the vendor after deducting the commission. Your shop and its dishes are visible to customers while your shop is active. We may suspend or remove a shop that breaks these terms, gives false information, or harms customers.",
						"You are responsible for the accuracy of your menu, prices and delivery details, for preparing food safely and legally, for fulfilling the orders you accept, and for dealing fairly with customers who contact you about them.",
						"Each order is placed with one vendor. The setup fee is not refundable unless we say otherwise in writing.",
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
