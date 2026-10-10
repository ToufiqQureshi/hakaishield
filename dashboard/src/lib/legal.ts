// Increment these identifiers whenever the corresponding public document
// changes in a material way. Signup stores the exact versions accepted.
export const LEGAL_VERSIONS = {
  terms: '2026-10-05',
  privacy: '2026-10-05',
  marketing: '2026-10-05',
};

export type LegalSection = { heading: string; paragraphs?: string[]; bullets?: string[] };

export const termsSections: LegalSection[] = [
  { heading: '1. Who we are and these terms', paragraphs: [
    'These Terms govern your access to the HakaiShield website, dashboard, APIs, and traffic protection services (the “Service”). “HakaiShield”, “we”, “us”, and “our” refer to the operator of the Service. Before paid service is offered, the operator’s full legal name, registered address, and contact details must be shown here and on your order or invoice.',
    'By creating an account or using the Service, you agree to these Terms and the Privacy Notice. If you use the Service for a company, you confirm that you may bind that company. If you do not agree, do not create an account or use the Service.',
  ] },
  { heading: '2. Accounts and security', paragraphs: [
    'Provide accurate account details, keep your credentials and recovery methods secure, and promptly tell us about suspected unauthorized access. You are responsible for activity under your account except to the extent caused by our failure to meet obligations that apply to us.',
    'You must be legally able to enter into a contract where you live. The Service is intended for organizations and adults acting for an organization; it is not designed for children.',
  ] },
  { heading: '3. Domains and your responsibilities', paragraphs: [
    'You may connect only domains and origin systems that you own or are authorized to administer. We may ask you to prove control before enabling a domain. You remain responsible for your website, origin, content, user notices, lawful collection and use of data, backups, and compliance with laws and third-party terms.',
    'You must configure DNS and origin settings exactly as instructed, test changes, keep a working origin, and maintain a rollback path. Do not submit sensitive personal data, payment card data, passwords, or special-category data to the Service unless we have expressly agreed in writing to handle it.',
  ] },
  { heading: '4. What the Service does and its limits', paragraphs: [
    'HakaiShield analyzes network and request signals and may allow, challenge, rate-limit, deceive, or block traffic according to the features and settings actually enabled for your account. Detection can be wrong: legitimate visitors may be challenged or blocked, and malicious traffic may pass through. You must review decisions and test policies before enforcing them.',
    'Unless your order explicitly says otherwise, the Service is provided as configured and may include preview or shadow-only features that record a proposed decision without changing traffic. A dashboard setting or saved rule does not mean protection is active. Check the domain status and enforcement mode before relying on a control.',
    'The Service is one security layer, not a substitute for secure application design, a web application firewall, backups, incident response, or other controls appropriate to your risk. No security product can guarantee that a site will be available, attack-free, or free from loss.',
  ] },
  { heading: '5. Plans, charges, taxes, and cancellation', paragraphs: [
    'Any price, plan limits, currency, billing interval, included usage, overage, and applicable taxes will be displayed before you place an order. Your order and these Terms together describe the paid subscription. We will not charge you unless you complete checkout with the payment provider. We do not ask you to send card details by email or through a HakaiShield form.',
    'Unless the checkout says otherwise, subscriptions renew for the displayed billing interval until cancelled. You may cancel future renewal through the billing portal or by contacting support. Cancellation takes effect at the end of the paid period; charges already incurred and any expressly disclosed usage charges remain due. Refund rights required by applicable law are not limited by this section; any additional refund commitment will be stated at checkout.',
    'If a payment fails or your subscription expires, we may pause account features after reasonable notice. We will describe any effect on live traffic before you purchase. We will not silently represent an unpaid or expired domain as protected.',
    'You are responsible for taxes on your purchase except taxes on our income. The payment provider may process your payment under its own terms and privacy notice.',
  ] },
  { heading: '6. Acceptable use', paragraphs: [
    'You must follow the Acceptable Use Policy. In particular, do not use the Service to attack systems, evade access controls, conduct unlawful surveillance, interfere with emergency or critical services, or harm people. We may investigate credible abuse and suspend access when reasonably needed to protect people, systems, customers, or legal rights.',
  ] },
  { heading: '7. Your data and feedback', paragraphs: [
    'You retain rights to content and configuration you submit. You authorize us and our service providers to process them only as needed to provide, secure, support, and improve the Service as described in the Privacy Notice and any data-processing agreement that applies to your order.',
    'You grant us a limited, non-exclusive right to host and process submitted material for those purposes. You may send feedback; we may use it without restriction or payment, provided we do not disclose your confidential information.',
  ] },
  { heading: '8. Availability and changes', paragraphs: [
    'We will use reasonable care to operate the Service, but do not promise uninterrupted or error-free availability. We may change or discontinue features. For a material adverse change to a paid feature, we will give reasonable advance notice where practical and explain any available cancellation or remedy required by law.',
    'We may update these Terms. For a material change, we will provide notice before it applies where practical. If the law requires renewed consent, we will ask for it. Continued use after an effective change means acceptance only where applicable law permits that approach.',
  ] },
  { heading: '9. Suspension and termination', paragraphs: [
    'You may stop using the Service and cancel your subscription as described above. We may suspend or terminate access for material breach, non-payment, a security emergency, or a legal requirement. Where practical, we will give notice and an opportunity to address the issue. On termination, access ends and data is handled under the Privacy Notice, retention settings, and applicable law.',
  ] },
  { heading: '10. Confidentiality and security', paragraphs: [
    'Each party will use reasonable care to protect the other party’s non-public information and use it only for the relationship. We maintain technical and organizational safeguards appropriate to the Service. No transmission or storage method is perfectly secure.',
  ] },
  { heading: '11. Warranties and liability', paragraphs: [
    'To the extent allowed by law, the Service is provided without implied warranties that cannot be excluded. Nothing in these Terms excludes liability that cannot lawfully be excluded, including applicable consumer rights, fraud, or intentional misconduct.',
    'To the extent allowed by law, neither party is liable for indirect or consequential loss. To the extent allowed by law, each party’s total liability arising from the Service is limited to the fees you paid or owe for the Service during the twelve months before the event giving rise to the claim. This limit does not apply to liability that cannot lawfully be limited.',
  ] },
  { heading: '12. Governing law and contact', paragraphs: [
    'The governing law, dispute venue, and mandatory consumer protections depend on the operator’s place of establishment and your location. The operator’s legal name, address, governing law, and formal notices contact must be completed in this section before paid subscriptions are offered. Nothing here removes rights that cannot lawfully be waived.',
    'For questions or notices, use the support contact published on our Contact page. A working support and legal-notices address must be configured before launch.',
  ] },
];

export const privacySections: LegalSection[] = [
  { heading: '1. Who controls your information', paragraphs: [
    'This notice explains how the operator of HakaiShield processes personal information when you visit our website, create an account, contact us, or configure the Service. The operator’s legal name, address, and privacy contact must be identified here before public paid launch. When we process visitor data for a customer’s site, that customer generally decides why it is processed and is responsible for its own notices; we process it to provide the Service.',
  ] },
  { heading: '2. Information we process', bullets: [
    'Account information: name, email, company, authentication identifiers, and account settings supplied through signup or account use.',
    'Customer configuration: domain names, origin targets, verification state, protection policies, and support communications.',
    'Security and request signals: TLS fingerprints such as JA4, selected request headers and protocol signals, paths or route classes needed for security decisions, timestamps, and the resulting score, signals, and action. The proxy may handle the visitor IP address transiently to route traffic and calculate rate limits. Durable decision evidence is configured for a 30-day default retention period in the backend; operators may configure this between 1 and 365 days. In-memory evidence is bounded and normally limited to 24 hours.',
    'Optional browser beacon data: if a customer installs the beacon, it may report coarse interaction flags such as pointer, scroll, keyboard, or touch activity. It is not proof of a human and should only be enabled with appropriate notice and legal basis.',
    'Operational data: service logs, diagnostic events, approximate usage and billing status once billing is available, and messages you send to support.',
  ] },
  { heading: '3. Why we use information', bullets: [
    'Create and secure accounts, authenticate users, and provide requested features.',
    'Route and protect customer traffic, detect abuse, troubleshoot incidents, and maintain service reliability.',
    'Respond to support requests, communicate important service or security notices, and administer subscriptions when enabled.',
    'Meet legal obligations, enforce our terms, and establish or defend legal claims.',
    'Send product marketing only where permitted. You can opt out at any time; service and security notices are not marketing.',
  ] },
  { heading: '4. Legal grounds and choices', paragraphs: [
    'Where data protection law requires a legal ground, the operator will rely on the ground appropriate to the purpose, such as providing a requested service, legitimate interests in securing and operating the Service, consent where required, or a legal obligation. You can withdraw consent for future processing where consent is the basis. Withdrawal does not make prior processing unlawful.',
    'Do not put personal or sensitive information into domain names, route labels, rules, or support messages unless necessary and authorized.',
  ] },
  { heading: '5. Sharing and service providers', paragraphs: [
    'We share information with infrastructure, database, authentication, email, payment, and support providers only as needed to operate the Service, and with professional advisers or authorities when legally required or necessary to protect rights and safety. Provider names, processing locations, and applicable transfer safeguards must be listed here or in an accessible provider list before paid launch. We do not sell personal information for money.',
  ] },
  { heading: '6. International transfers', paragraphs: [
    'The Service is intended for international use, so information may be processed in countries different from yours. Before launch, we will identify the operator’s country, hosting regions, and transfer safeguards used where required by law. Contact us for the safeguards applicable to your account.',
  ] },
  { heading: '7. Retention and deletion', paragraphs: [
    'We keep account and configuration information while the account is active and as needed for support, security, legal obligations, and dispute resolution. Durable traffic evidence defaults to 30 days when that feature is enabled; in-memory evidence is bounded and normally expires after 24 hours. Training samples have a 30-day default. Backups and security logs may persist for a limited additional period. Actual periods and deletion processes must be configured and verified for each production environment before launch.',
    'You can request account closure or deletion through the published support contact. We will delete or de-identify information when no longer needed, subject to legal retention duties and secure backup expiry.',
  ] },
  { heading: '8. Your rights', paragraphs: [
    'Depending on where you live, you may have rights to access, correct, delete, restrict, object to, or receive a copy of personal information, withdraw consent, or complain to a privacy regulator. Send requests to our published privacy contact. We may need to verify your identity and may retain information we must keep by law.',
  ] },
  { heading: '9. Cookies and local storage', paragraphs: [
    'The dashboard uses browser storage needed for authentication and remembering interface preferences. We do not intentionally use advertising cookies. If analytics or other non-essential tracking is introduced, we will update this notice and provide any consent controls required by law before enabling it.',
  ] },
  { heading: '10. Security and children', paragraphs: [
    'We use safeguards intended to protect information, but no system is completely secure. Tell us promptly if you believe an account or customer domain has been compromised. The Service is not directed to children and we do not knowingly seek to collect children’s personal information.',
  ] },
  { heading: '11. Changes and contact', paragraphs: [
    'We may update this notice as the Service or legal requirements change. We will update the date and, for material changes where required, provide notice. For privacy questions or requests, use the privacy contact published on our Contact page. A monitored privacy contact must be configured before launch.',
  ] },
];

export const acceptableUseSections: LegalSection[] = [
  { heading: 'Use only on systems you control', paragraphs: ['Use HakaiShield only for domains and origins you own or have clear authorization to administer. Keep proof of authorization and promptly remove domains when your authority ends.'] },
  { heading: 'Prohibited conduct', bullets: [
    'Do not use the Service to launch, coordinate, conceal, or facilitate denial-of-service attacks, credential theft, malware, phishing, unauthorized scanning, intrusion, or evasion of another party’s access controls.',
    'Do not proxy, challenge, deceive, or block traffic for unlawful discrimination, harassment, stalking, political repression, or other unlawful surveillance or harm.',
    'Do not interfere with emergency response, healthcare, critical infrastructure, or systems where disruption could create a risk to life or safety.',
    'Do not probe, overload, reverse engineer to attack, or attempt unauthorized access to HakaiShield, its infrastructure, other customers, or third-party systems.',
    'Do not upload unlawful content or personal data you lack authority to process; do not use the Service to evade sanctions or applicable export controls.',
  ] },
  { heading: 'Enforcement and reporting', paragraphs: ['We may investigate credible reports, preserve relevant records, restrict features, or suspend an account where reasonably necessary to address abuse, protect the Service or others, or comply with law. Where practical, we will provide notice and a route to appeal. Report suspected abuse through the Contact page.'] },
];

export const cookieSections: LegalSection[] = [
  { heading: 'Current browser storage', paragraphs: [
    'HakaiShield currently uses browser storage required for sign-in/session handling and a local preference for the dashboard theme. Authentication storage is necessary to provide the account feature you request. We do not intentionally set advertising cookies or cross-site advertising trackers.',
    'You can clear site storage in your browser, but doing so may sign you out or reset preferences. If we add analytics or other non-essential tracking, this notice and the privacy notice will be updated and any consent required by law will be requested before those tools run.',
  ] },
];

export const dpaSections: LegalSection[] = [
  { heading: '1. Scope and roles', paragraphs: [
    'This Data Processing Addendum (“DPA”) applies when a customer uses HakaiShield to process personal data in traffic to the customer’s domains. The customer is the controller (or equivalent business) and HakaiShield is the processor (or service provider), except for account, billing, security, and service administration data that HakaiShield processes for its own purposes.',
    'The DPA becomes binding only when the customer has a paid order or other written agreement with the identified HakaiShield legal entity. That entity and its address must be completed before launch. This page is a proposed DPA and must be reviewed for the applicable countries before it is offered as a signed contract.',
  ] },
  { heading: '2. Processing instructions and purpose', paragraphs: [
    'We process customer personal data only on documented customer instructions, including operating the proxy, assessing request signals, applying configured security rules, providing evidence and support, and complying with legal requirements. The customer is responsible for the lawfulness of its instructions, its own privacy notices, and selecting appropriate settings.',
  ] },
  { heading: '3. Data and duration', paragraphs: [
    'The processing may concern website visitors, account users, and support contacts. Data may include online identifiers and request metadata, TLS fingerprints, selected headers, route or request classifications, timestamps, security signals and outcomes, and optional coarse interaction flags when the customer installs the browser beacon. The Service is not designed to process sensitive personal data or payment card information in customer traffic.',
    'Processing lasts for the customer’s use of the Service. Durable evidence defaults to 30 days when enabled, in-memory evidence is bounded and normally limited to 24 hours, and training samples default to 30 days. Production settings, backup expiry, and deletion behavior must be confirmed in the applicable order or current subprocessor documentation.',
  ] },
  { heading: '4. Processor commitments', bullets: [
    'Process customer personal data only for the Service, the customer’s documented instructions, or a legal requirement; notify the customer if we believe an instruction violates applicable data protection law.',
    'Ensure personnel authorized to process the data are subject to confidentiality obligations.',
    'Maintain technical and organizational security measures appropriate to the risks, and provide information reasonably needed to demonstrate compliance with this DPA.',
    'Taking account of the nature of processing, assist the customer through appropriate technical and organizational measures with data subject requests and the customer’s security, breach, impact assessment, and regulator consultation obligations.',
    'Notify the customer without undue delay after becoming aware of a personal data breach affecting customer data, with information reasonably available to support the customer’s response.',
    'At the customer’s choice, delete or return customer personal data after the Service ends, except where law requires retention and subject to secure deletion from backups under documented backup cycles.',
  ] },
  { heading: '5. Subprocessors', paragraphs: [
    'The customer gives general authorization to use subprocessors needed to operate the Service, subject to written data protection obligations no less protective than this DPA. We remain responsible for their performance as required by law. Before paid launch, we will publish the actual subprocessor legal entities, service, processing locations, and a notice period and objection process for material changes. Providers merely planned for future hosting are not represented as currently processing customer data.',
  ] },
  { heading: '6. International transfers', paragraphs: [
    'We will not transfer customer personal data internationally unless a lawful transfer mechanism and any required supplementary safeguards are in place. The applicable hosting regions, transfer mechanism, and executed standard contractual clauses or other safeguards must be identified in the customer’s order or transfer documentation before the transfer occurs.',
  ] },
  { heading: '7. Audits and precedence', paragraphs: [
    'On reasonable written request, we will provide information needed to demonstrate compliance and permit proportionate audits, subject to confidentiality, security, and other customers’ rights. The parties will first use available independent reports where appropriate. If this DPA conflicts with the main Terms concerning personal data processing, this DPA controls for that subject. Mandatory statutory rights remain unaffected.',
  ] },
];
