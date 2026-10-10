# HakaiShield — ab kya baaki hai (7 Oct 2026)

**Status: public paid launch NO-GO.** Fresh signup → session → authenticated dashboard/API pass. Domain ownership → live HTTPS → protected traffic abhi live test nahi hua. Payment aur model training aapki preference ke mutabik last phases hain.

| Order | Kaam | Main kya karunga | Aap se kya chahiye |
| --- | --- | --- | --- |
| 1 | Customer domain ka live E2E | TXT ownership, DNS A, ACME HTTPS, origin proxy, monitor evidence aur rollback test | Ek controlled hostname aur uska DNS edit access; existing live traffic hai ya nahi batayein |
| 2 | Production edge | AWS par Go edge, stable Elastic IP, persistent cert storage, TLS ports, health checks aur Cloudflare Pages dashboard deploy/test | AWS account access/billing; dashboard ke liye domain/Cloudflare access |
| 3 | Reliable paid-service operation | Durable usage metering, certificate/queue/origin alerts, dependency advisories, backup-restore, load test, failover aur deploy rollback | Hosting budget aur acceptable outage/retention requirements |
| 4 | Account recovery & support | Password-reset delivery, support contact aur recovery flow test | Verified email sender domain + SMTP provider credentials; email confirmation off reh sakti hai |
| 5 | Legal + payments **last** | Operator details ke saath legal pages final, plan/price, hosted checkout, verified webhooks, entitlements, cancellation/recovery test | Registered business country/entity, legal identity/contact, payment provider account, final prices/tax/refund policy |
| 6 | Model training **last** | Real labelled traffic par offline evaluation, shadow deployment aur false-positive review | Representative traffic aur approved labels |

**DNS ke bina protection:** current reverse proxy ko protected hostname ka traffic edge tak aana zaruri hai. `cloudflared` tunnel customer TLS ko pehle terminate kar dega, isliye current JA4/ACME edge flow us par nahi chalega. Cloudflare Worker connector alag implementation hai; JavaScript snippet sirf observation dega, full blocking nahi. Isko direct-edge launch ke baad evaluate karna hai.

**Next immediate input:** `shield.interviewyaar.lol` ka DNS aap edit kar sakte hain? Is par abhi koi live site chal rahi hai? Agar nahi, ek doosra hostname dein jiska DNS aap control karte hain.

## AWS + Cloudflare access: minimum handoff

Repo ke purane runbook mein Mumbai EC2 `i-04a0ded0c9d9c7434` listed hai; owner ne ise active bataya. Cloudflare login ab connected hai aur `hakaishield-dashboard` Pages project account mein confirm hua. AWS profile aur documented SSH-key path is machine par nahi mile.

| Service | Aap kya confirm/setup karein | Kyun |
| --- | --- | --- |
| AWS | EC2 instance abhi aapke account mein hai? Region `ap-south-1` confirm karein. Is machine par AWS CLI SSO/session access **ya** EC2 ke liye SSM Session Manager setup karein; SSH use karna ho to private key local private folder mein rakhein, chat mein na bhejein. | Existing server inspect aur deploy karna. |
| AWS | EC2 par stable Elastic IP allocate/associate karne aur security group mein public 443, certificate renewal ke liye 80, restricted admin access configure karne ki permission. | Customer DNS ko stable edge IP chahiye. |
| Cloudflare | Wrangler login aur Pages project confirmed. Current OAuth token mein Pages write/zone read hai; DNS changes ke liye relevant zone ka scoped DNS Write token ya Cloudflare console action chahiye. Token chat mein na bhejein. | Controlled DNS TXT/A test; dashboard preview/prod deployment mein Pages access already available hai. |
| Hostnames | Staging hostname, uska current traffic status, origin URL, aur dashboard ka final HTTPS URL batayein. | Safe DNS cutover, origin/TLS test aur API CORS config. |
| ACME contact | Existing working inbox use kar sakte hain; naya domain inbox zaruri nahi. | Certificate authority account contact. Certificate-expiry alert apne monitoring mein configure karna hoga. |

Customer TLS/JA4 ke liye protected hostname ka DNS **DNS-only** hoga aur traffic direct AWS edge par jayega. Cloudflare Pages dashboard ke liye hai; `cloudflared` tunnel current customer traffic path mein use nahi hoga.
