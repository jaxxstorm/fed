package guidancecontent

import (
	"github.com/jaxxstorm/fed/internal/readiness"
	"io/fs"
)

// AFC authors provider communication duties and preserves FedRAMP-only exclusions.
func AFC(rules, markdown fs.FS) (readiness.Package, error) {
	return administrativeDraft("AFC", rules, markdown, map[string]administrativeRecipe{
		"AFC-CSO-INB": {
			"Keep FedRAMP's security contact usable through personnel and mailbox changes; a listed address without a working inbox is insufficient.",
			"Provider security communications owner and mail administrator",
			"Locate the Marketplace Security Email, the actual receiving mailbox and responsible security/compliance staff; reconcile any separately notified replacement address.",
			"Establish the FSI address and ownership, delivery path and staff access. Document continuity when individuals change; compare the address with the Marketplace or prior notification.",
			"Maintain the receiving inbox and investigate delivery or access failures. Notify an address change through AFC-CSO-NOC rather than assuming a local alias updates FedRAMP's records.",
			"the FSI address, mailbox configuration and ownership history, Marketplace/contact record, and dated delivery/access observations",
			"Compare the address FedRAMP uses with the actual mailbox and delivery observations; assess continued access through staffing changes.",
			"The security contact and receiving path for this exact cloud service offering, including forwarding destinations.",
			"Establish for the applicable certification and maintain during operation; retain actual change and delivery dates. The rule supplies no numeric mailbox test interval.",
			"A personal contact name or Marketplace listing without evidence the mailbox receives messages and remains maintained.",
		},
		"AFC-CSO-NOC": {
			"Keep FedRAMP's destination current when the FSI email address changes.", "Provider security communications owner",
			"Identify the old and new FSI addresses, actual change time and person authorized to submit the CSP Notification of Changes form.",
			"Include the FedRAMP notification in the address-change workflow and retain the official form destination and submission receipt.",
			"Immediately notify FedRAMP of each FSI address change using the published CSP Notification of Changes form.",
			"the old/new address change record, actual effective time, submitted notification content and timestamped receipt",
			"Reconcile every address change to its FedRAMP notification; compare timestamps to the immediate-notification requirement without inventing an hourly allowance.",
			"All changes to this service's FedRAMP Security Inbox email address.",
			"Immediately on any change to the FSI email address; retain the change time and submission time. No numeric substitute for immediately is specified.",
			"A change ticket or internal announcement without the notification to FedRAMP.",
		},
		"AFC-CSO-TFG": {
			"Route government-origin messages under the FedRAMP default while retaining the exception for confirmed non-FedRAMP origin.", "Provider mail administrator and security communications owner",
			"Locate FSI mail handling rules, sender authentication results, and the method used to confirm whether a government sender is outside FedRAMP.",
			"Configure handling for both @fedramp.gov and @gsa.gov. Do not require a sender display-name tag: FedRAMP notes that FedRAMP or F20B in names is not universally enforceable.",
			"Treat mail originating from either government domain as FedRAMP by default. Stop applying FSI rules to a particular message only after confirming that its origin is someone other than FedRAMP.",
			"mailbox rules, automated configuration validation, full message headers, handling records and attributable non-FedRAMP confirmations where used",
			"Test both domains and trace any exception to affirmative origin confirmation. Distinguish actual origin from an unverified display name; do not narrow the default to known staff.",
			"Messages originating from both official domains received by this service's FSI, including exception handling.",
			"Apply on receipt; preserve receipt, routing and origin-confirmation times. This rule sets no numeric processing deadline.",
			"A sender allowlist limited to known names or a claim that all @gsa.gov mail is outside FedRAMP.",
		},
		"AFC-CSO-RCV": {
			"Ensure government messages reach security staff and receive a reaction without customer-portal or challenge-response barriers.", "Provider mail administrator and security operations owner",
			"Map ingress filters, quarantine, forwarding, ticket creation and the staff reaction path from the FSI.",
			"Remove requirements for FedRAMP to solve CAPTCHAs, authenticate to customer portals, or take service-specific actions before delivery; arrange staff ownership of received messages.",
			"Receive and react to FedRAMP messages without disruption or additional action by FedRAMP; investigate blocked or stranded messages along the full path.",
			"end-to-end delivery observations, message headers, filter and routing settings, and attributable security staff reaction records",
			"Trace delivery through the mailbox into a real reaction. A successful SMTP delivery alone does not establish staff handling or absence of downstream barriers.",
			"The complete FedRAMP-to-security-team path, including third-party mail or ticket systems and forwarding.",
			"During receipt and handling of FedRAMP messages; retain actual delivery and reaction times. Apply separate designated-message deadlines where relevant, not an invented SLA here.",
			"An inbox address, SMTP success, or portal auto-reply that still requires FedRAMP to take another action.",
		},
		"AFC-CSO-CRA": {
			"Execute Emergency and Emergency Test instructions by their message-specific deadlines; acknowledging an email is not completing its actions.", "Provider security response lead",
			"Retain the original designated message, its instructions, service class and explicit action timeframes; identify the staff able to complete each action.",
			"Map each requested action to an owner and the actual deadline in the message. Keep estimated-resolution reporting distinct from final resolution.",
			"Complete all required actions in each Emergency or Emergency Test message within the timeframe included in that message; preserve failures as well as completed actions.",
			"the original message with sender, subject, body and receipt time, action-to-owner mapping, execution records, and timestamped responses/results",
			"Compare completion of each requested action with its own stated deadline. AFC-FRP-ERT defaults are for providing an estimated resolution time, not universal remediation deadlines: Class D within 12 hours; C by 3:00 p.m. Eastern Time on the 2nd business day; B on the 3rd business day; A on the 5th business day. The message may require faster action.",
			"Every required action in the Emergency and Emergency Test messages for the selected service and class.",
			"Within the timeframe included in the message. Preserve time zone, business-day calculation where applicable, receipt time and actual completion time; use the message's instructions rather than replacing them with a default.",
			"An automated acknowledgment or estimated resolution time asserted as proof that all required actions were completed.",
		},
		"AFC-CSO-EMR": {
			"Make a provider-designated senior security official aware of Emergency messages.", "Provider security communications owner",
			"Identify the senior security official selected by the provider and the FSI's Emergency-message routing path.",
			"Configure and validate Emergency subject routing to that official, including current recipient information and forwarding behavior.",
			"Route Emergency designated messages sent by FedRAMP to the senior security official for awareness.",
			"the provider's official designation, mailbox routing configuration, automated configuration validation and timestamped forwarding/delivery records",
			"Confirm the recipient is the provider-designated senior security official and trace actual Emergency messages to that recipient. A generic support queue is insufficient.",
			"Emergency designated messages and the named senior security official; this clause does not independently extend the routing mandate to every message type.",
			"When an Emergency designated message is received. Retain receipt and routing times; no numeric routing interval is stated.",
			"A routing diagram without a current recipient, or delivery only to a general support queue.",
		},
		"AFC-CSO-IMA": {
			"Handle Important messages according to their instructions while preserving SHOULD rather than converting it to MUST.", "Provider security response lead",
			"Locate original Important messages, their required actions and specified timeframes, and any documented decision not to follow the recommendation.",
			"Assign Important-message actions and preserve message-specific due times separately from Emergency handling.",
			"Complete the actions in Important designated messages within the specified message timeframe, or record the reasons and implications for a departure.",
			"original Important messages, action records with completion times, responses, and documented dispositions for departures",
			"Compare each action and completion time to the source message. Assess documented departure rationale without treating SHOULD as optional silence or absolute MUST.",
			"Important designated messages for this service; ordinary messages without designators are general communications.",
			"Within the timeframe specified in each Important message; timeframes may vary by certification class.",
			"A message acknowledgment without action completion or a documented recommendation disposition.",
		},
		"AFC-CSO-ACK": {
			"Let FedRAMP know its message reached the FSI without confusing acknowledgment with action completion.", "Provider mail administrator",
			"Locate the FSI receipt records and acknowledgment mechanism or documented reason for declining automatic acknowledgment.",
			"Configure a receipt-triggered automatic acknowledgment and verify it is emitted for FedRAMP messages without requiring a staff click.",
			"Promptly and automatically acknowledge messages received from FedRAMP in the FSI.",
			"acknowledgment configuration, incoming message IDs and times, correlated outgoing automatic acknowledgment IDs and times, and any documented departure rationale",
			"Correlate receipt to automatically emitted acknowledgment and evaluate unnecessary delay. Manual acknowledgments do not demonstrate automation.",
			"FedRAMP messages received in the selected service's FSI, not only emergencies.",
			"Promptly means without unnecessary delay. Retain actual receipt and acknowledgment timestamps; no fixed number of minutes is imposed.",
			"An auto-reply template without execution records, or a manual reply described as automatic.",
		},
	})
}
