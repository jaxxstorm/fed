import {expect,test} from "@playwright/test";

test("issued agent token remains visible when settings gains a fifth form", async ({page}) => {
 let issued = 0;
 const forms = ["preview-profile", "scope", "token", "destination"].map(action => ({
  Action:`/services/fixture/actions/${action}`, Title:action === "token" ? "Agent access" : action,
  Button:action === "token" ? "Issue scoped token" : "Save", Fields:[],
 }));
 await page.route("**/api/ui/v1/**", async route => {
  if (route.request().url().endsWith("bootstrap")) return route.fulfill({json:{services:[{id:"fixture",name:"Token fixture"}],csrf:"test",capabilities:{fixture:["read","admin"]}}});
  if (route.request().method() === "POST") {
   issued++;
   return route.fulfill({json:{Title:"Agent token",Notice:"Copy this token now; it is only shown once.",Sections:[{Title:"Token",Text:`test-secret-token-${issued}`} ]}});
  }
  return route.fulfill({json:{Title:"Settings",Forms:[...forms,...Array.from({length:issued},(_,i)=>({Action:"/services/fixture/actions/revoke-token",Title:`Issued token ${i+1}`,Button:"Revoke this token",Fields:[{Name:"hash",Type:"hidden",Value:`hash-${i}`}]}))]}});
 });
 await page.goto("/services/fixture?page=Settings");
 for (let i=1;i<=2;i++) {
  await page.getByRole("button",{name:"Issue scoped token",exact:true}).click();
  await expect(page.locator("summary").filter({hasText:`Issued token ${i}`})).toBeVisible();
  await expect(page.getByText(`test-secret-token-${i}`,{exact:true})).toBeVisible();
  await expect(page.getByText("Copy this token now; it is only shown once.")).toBeVisible();
 }
 expect(await page.evaluate(()=>JSON.stringify([localStorage,sessionStorage]))).not.toContain("test-secret-token");
 await page.reload();
 await expect(page.getByRole("button",{name:"Issue scoped token",exact:true})).toBeVisible();
 await expect(page.getByText("test-secret-token-2",{exact:true})).toHaveCount(0);
});

test("blocked requirement explains recovery before evidence details",async({page})=>{
 await page.route("**/api/ui/v1/**",route=>route.fulfill({json:route.request().url().endsWith("bootstrap")?{services:[{id:"fixture",name:"Recovery"}],csrf:"test",capabilities:{fixture:["read"]}}:{Title:"KSI-SVC-ACM",Detail:true,Primary:{status:"blocked",action:"How to unblock this requirement",reason:"GitHub access is missing.",secondary:[],next_steps:[{title:"GitHub access is missing",explanation:"Authorize the installation, test access, then rerun collection.",action:"Configure repository access",provider:"github",targets:["example/repo@commit:workflow.yaml"]},{title:"Evidence still needed",explanation:"Dated drift-review records are missing.",action:"Submit exact citations",targets:[]}]},Sections:[],Forms:[]}}));
 await page.goto("/services/fixture/requirements/KSI-SVC-ACM");
 await expect(page.getByRole("heading",{name:"How to unblock this requirement"})).toBeVisible();
 await expect(page.getByText("Dated drift-review records are missing.")).toBeVisible();
 await expect(page.getByRole("link",{name:"Open Github connections"})).toHaveAttribute("href","/services/fixture?page=Connections&provider=github");
 await page.getByText("1 related targets or sources").click();
 await expect(page.getByText("example/repo@commit:workflow.yaml")).toBeVisible();
});

test("incomplete onboarding identifies setup instead of showing completion",async({page},info)=>{
 await page.route("**/api/ui/v1/**",route=>route.fulfill({json:route.request().url().endsWith("bootstrap")?{services:[{id:"fixture",name:"New assessment for a long service name across production and development environments"}],csrf:"test",capabilities:{fixture:["read","admin"]}}:{as_of:"2026-10-16T12:00:00Z",setup_required:true,rules:[],counts:{required:0},categories:{}}}));
 await page.goto("/services/fixture");
 await expect(page.getByRole("heading",{name:"Choose a profile to begin"})).toBeVisible();
 await expect(page.getByRole("link",{name:"Set up this service"})).toHaveAttribute("href","/services/fixture?page=Settings");
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await page.screenshot({path:info.outputPath("Onboarding.png")});
});

test("source validation and concurrent edits preserve inputs",async({page})=>{
 let revision=1;const submitted:URLSearchParams[]=[];
 const source={type:"document",name:"Security policies",locator:"https://docs.example/security",scope_revision:1,enabled:true,freshness_hours:48,paths:[],requirement_ids:[]};
 await page.route("**/api/ui/v1/**",async route=>{
  const url=route.request().url();
  if(url.endsWith("bootstrap"))return route.fulfill({json:{services:[{id:"fixture",name:"Fixture service",organization_id:"org"}],csrf:"test",capabilities:{fixture:["read","edit","admin"]},can_create:true}});
  if(route.request().method()==="POST"){
   submitted.push(new URLSearchParams(route.request().postData()||""));
   if(submitted.length===1)return route.fulfill({status:400,json:{code:"invalid_input",title:"Check these inputs",explanation:"Correct the source location",field_errors:{locator:"Provide a valid source URL."}}});
   if(submitted.length===2){revision=2;return route.fulfill({status:409,json:{code:"revision_conflict",title:"Changes were saved elsewhere",explanation:"Compare the current revision before retrying."}})}
   revision=3;return route.fulfill({json:{redirect:"/services/fixture?page=Sources"}});
  }
  return route.fulfill({json:{Title:"Fixture service",Notice:"",Primary:null,Citations:null,Forms:[{Action:"/services/fixture/actions/evidence-source",Title:"Source · Security policies",Button:"Save source",CSRF:"test",Fields:[{Name:"id",Type:"hidden",Value:"policies",Label:""},{Name:"expected_revision",Type:"hidden",Value:String(revision),Label:""},{Name:"payload",Type:"textarea",Value:JSON.stringify(source),Label:"Source configuration"}]}]}});
 });
 await page.goto("/services/fixture?page=Sources");
 await page.getByLabel("Name",{exact:true}).fill("Updated policy source");
 await page.getByRole("button",{name:"Save source"}).click();
 await expect(page.getByText("Provide a valid source URL.")).toBeVisible();
 await expect(page.getByLabel("Name",{exact:true})).toHaveValue("Updated policy source");
 await page.getByLabel("Locator",{exact:true}).fill("https://docs.example/approved");
 await page.getByRole("button",{name:"Save source"}).click();
 await expect(page.getByText("Changes were saved elsewhere")).toBeVisible();
 await expect(page.getByLabel("Name",{exact:true})).toHaveValue("Updated policy source");
 await page.getByRole("button",{name:/I compared the changes/}).click();
 await page.getByRole("button",{name:"Save source"}).click();
 await expect(page.getByText("Saved successfully.")).toBeVisible();
 expect(submitted[2].get("expected_revision")).toBe("2");
 expect(JSON.parse(submitted[2].get("payload")||"{}").name).toBe("Updated policy source");
});

test("expired sessions offer sign-in instead of empty data",async({page})=>{
 await page.route("**/api/ui/v1/bootstrap",route=>route.fulfill({status:401,json:{code:"session_expired",title:"Sign in to continue",explanation:"Your session expired."}}));
 await page.goto("/");await expect(page.getByRole("heading",{name:"Sign in to continue"})).toBeVisible();await expect(page.getByRole("link",{name:"Sign in",exact:true})).toHaveAttribute("href","/auth/login");
});

test("failed refresh retains successful evidence and its timestamp",async({page})=>{
 let fail=false;
 await page.route("**/api/ui/v1/**",route=>{
  if(route.request().url().endsWith("bootstrap"))return route.fulfill({json:{services:[{id:"fixture",name:"Evidence fixture"}],csrf:"test",capabilities:{fixture:["read"]}}});
  if(fail)return route.fulfill({status:503,json:{code:"unavailable",title:"Collection service unavailable",explanation:"Retry shortly."}});
  return route.fulfill({json:{items:[{id:"evidence",revision:1,created_at:"2026-10-16T12:00:00Z",data:{description:"Successful production read",source:"AWS",verification:"agent_observation"}}],offset:0,has_more:false}});
 });
 await page.goto("/services/fixture?page=Evidence");
 await expect(page.getByRole("heading",{name:"Successful production read"})).toBeVisible();
 fail=true;await page.getByRole("button",{name:"Refresh records"}).click();
 await expect(page.getByRole("alert")).toContainText("Collection service unavailable");
 await expect(page.getByRole("heading",{name:"Successful production read"})).toBeVisible();
 await expect(page.getByText(/Showing results from/)).toBeVisible();
});

test("review cites immutable evidence and preserves notes on conflict",async({page},info)=>{
 let revision=1;let attempts=0;let posted:URLSearchParams|undefined;
 const evidence={id:"artifact",revision:1,service_id:"fixture",kind:"evidence",created_at:"2026-10-16T12:00:00Z",data:{description:"Approved deployment policy",artifact_key:"stored",source:"Repository"}};
 await page.route("**/api/ui/v1/**",async route=>{
  const url=route.request().url();
  if(url.endsWith("bootstrap"))return route.fulfill({json:{services:[{id:"fixture",name:"Review fixture"}],csrf:"test",capabilities:{fixture:["read","review"]}}});
  if(url.includes("/preview/")){expect(url).toContain("revision=1");return route.fulfill({json:{content:"Policy\nAll changes are reviewed.\n"}})}
  if(route.request().method()==="POST"){
   posted=new URLSearchParams(route.request().postData()||"");attempts++;
   if(attempts===1){revision=2;return route.fulfill({status:409,json:{code:"revision_conflict",title:"Draft changed during review",explanation:"Inspect the latest draft before retrying."}})}
   return route.fulfill({json:{redirect:"/services/fixture/requirements/KSI-SVC-ACM?review=1"}});
  }
  return route.fulfill({json:{Title:"KSI-SVC-ACM · Configuration management",Detail:true,Sections:[{Title:"Assessment summary",Text:`Reviewed deployment policy · draft revision ${revision}`}],Citations:[{Evidence:evidence,StartLine:2,EndLine:2,JSONPointer:null,Label:"Agent result"}],Forms:[{Action:"/services/fixture/actions/review",Title:"Human review",Button:"Record review",Fields:[{Name:"requirement_id",Type:"hidden",Value:"KSI-SVC-ACM"},{Name:"revision",Type:"hidden",Value:String(revision)},{Name:"reviewed",Label:"I reviewed this implementation and its supporting evidence",Type:"checkbox",Value:""},{Name:"notes",Label:"Review notes",Type:"textarea",Value:""}]}]}});
 });
 await page.goto("/services/fixture/requirements/KSI-SVC-ACM?review=1");
 await page.getByRole("button",{name:/Agent result · Approved deployment policy/}).click();
 await expect(page.locator(".cited-line")).toHaveText("All changes are reviewed.");
 await page.screenshot({path:info.outputPath("Cited-review.png")});
 await page.getByRole("button",{name:"Close evidence"}).click();
 await page.getByLabel("Review notes").fill("Reviewed the policy and pinned workflow.");
 await page.getByLabel("I reviewed this implementation and its supporting evidence").check();
 await page.getByRole("button",{name:"Record review"}).click();
 await expect(page.getByRole("heading",{name:"Draft changed during review"})).toBeVisible();
 await expect(page.getByLabel("Review notes")).toHaveValue("Reviewed the policy and pinned workflow.");
 await expect(page.getByText("Reviewed deployment policy · draft revision 2")).toBeVisible();
 await page.getByRole("button",{name:/I compared the changes/}).click();
 await page.getByRole("button",{name:"Record review"}).click();
 await expect(page.getByText("Saved successfully.")).toBeVisible();
 expect(posted?.get("revision")).toBe("2");
});

test("exports retain snapshot context and permission failures stay explicit",async({page},info)=>{
 await page.route("**/api/ui/v1/**",route=>route.request().url().endsWith("bootstrap")?route.fulfill({json:{services:[{id:"fixture",name:"Export fixture"}],csrf:"test",capabilities:{fixture:["read"]}}}):route.fulfill({json:{Title:"Exports",Sections:[{Title:"Snapshot · 16 Oct 2026",Text:"Class B · 12 / 40 required requirements passing at preparation."}],Links:[{Text:"Download HTML · 16 Oct 2026",URL:"/services/fixture/exports/snapshot/html"}],Forms:[{Action:"/services/fixture/actions/prepare",Title:"Prepare assessment bundle",Button:"Prepare",Fields:[]}]}}));
 await page.route("**/services/fixture/exports/snapshot/html",route=>route.fulfill({contentType:"text/html",body:"<h1>Frozen assessment: 12 of 40</h1>"}));
 await page.goto("/services/fixture?page=Exports");
 await expect(page.getByText(/12 \/ 40 required requirements/)).toBeVisible();
 await expect(page.getByRole("button",{name:"Prepare",exact:true})).toBeDisabled();
 await page.screenshot({path:info.outputPath("Snapshot.png")});
 await page.getByRole("link",{name:/Download HTML/}).click();
 await expect(page.getByRole("heading",{name:"Frozen assessment: 12 of 40"})).toBeVisible();
 await page.route("**/api/ui/v1/services/fixture/pages/sources*",route=>route.fulfill({status:403,json:{code:"forbidden",title:"Additional access is needed",explanation:"Ask a service administrator for access."}}));
 await page.goto("/services/fixture?page=Sources");
 await expect(page.getByRole("alert")).toContainText("Additional access is needed");
});
