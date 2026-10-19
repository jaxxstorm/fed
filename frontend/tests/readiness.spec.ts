import { expect, test } from "@playwright/test";

test.afterEach(async ({ page }) => {
  expect(await page.pageErrors()).toEqual([]);
});

test("service work separates evidence from human review and resets pagination on filtering",async({page})=>{
  await page.route("**/api/ui/v1/**",route=>{
    const url=new URL(route.request().url());
    if(url.pathname.endsWith("bootstrap"))return route.fulfill({json:{services:[{id:"fixture",name:"Queue fixture"}],guidance_enabled:true,capabilities:{fixture:["read"]}}});
    if(url.pathname.endsWith("readiness/requirements")){
      const review=url.searchParams.get("group")==="review";
      if(review)expect(url.searchParams.get("cursor")).toBe("");
      return route.fulfill({json:{total:1,counts:{evidence:2,review:1},items:[{requirement_id:review?"REVIEW":"PROOF",name:review?"Recovery evidence review":"Production recovery evidence",applicability:"required",presentation:{label:review?"In review":"Needs evidence",reason:review?"Cited evidence awaits an authorized reviewer.":"Production records remain unsupported.",action:review?"Review the retained recovery assessment.":"Ask the recovery owner for production test records.",review_remaining:review,guidance_state:"missing"}}],next_cursor:review?undefined:"next-page"}});
    }
    return route.fulfill({json:{state:"missing",capabilities:[],questions:[],duties:[],total:0,question_count:0,duty_count:0,counts:{},items:[]}});
  });
  await page.goto("/services/fixture?page=Readiness");
  await expect(page.getByText("Next: Ask the recovery owner for production test records.")).toBeVisible();
  await page.getByRole("button",{name:"Next requirements",exact:true}).click();
  await page.getByLabel("Work group").selectOption("review");
  await expect(page.getByRole("link",{name:"Recovery evidence review",exact:true})).toHaveAttribute("href","/services/fixture/requirements/REVIEW");
  await expect(page.getByText("An authorized human must review the cited assessment. An agent result or declaration cannot grant approval.")).toBeVisible();
  await expect(page.getByRole("button",{name:"Previous requirements",exact:true})).toBeDisabled();
});

test("untriggered operating duty stays unassessed and retains cited timing",async({page})=>{
  let trigger="unknown";
  const duty={playbook_id:"response-plan",step_id:"report-event"};
  await page.route("**/api/ui/v1/**",route=>{
    if(route.request().url().endsWith("bootstrap"))return route.fulfill({json:{services:[{id:"fixture",name:"Duty fixture"}],csrf:"test",guidance_enabled:true,capabilities:{fixture:["read","edit"]}}});
    if(route.request().method()==="POST"){
      const body=route.request().postDataJSON();
      expect(body.data.duty).toEqual(duty);expect(body.data.capability_id).toBe("");expect(body.data.trigger).toBe("not_triggered");
      trigger=body.data.trigger;return route.fulfill({json:{revision:1}});
    }
    return route.fulfill({json:{service_id:"fixture",binding:{},state:"reviewed",total:0,question_count:0,capabilities:[],questions:[],offset:0,duty_count:1,duties:[{id:"duty",requirement_id:"RULE",duty,step:{id:"report-event",action:{text:"Retain the event report.",basis:"official_obligation"},owner_role:"Response owner",trigger:"event",timing:{text:"When the qualifying event occurs.",basis:"official_obligation"},completion_criteria:[]},implementation:"unknown",trigger,evidence_state:"missing",review:"not_assessed",needs_revalidation:false,next_action:"No triggering event does not mean the duty was completed."}]}});
  });
  await page.goto("/services/fixture?page=Readiness");
  await expect(page.getByText("Timing: When the qualifying event occurs. (Official Obligation)")).toBeVisible();
  await page.getByText("Record ownership or implementation declaration").click();
  await page.getByLabel("Operating trigger").selectOption("not_triggered");
  await page.getByRole("button",{name:"Save declaration",exact:true}).click();
  await expect(page.getByText("Declaration saved. Evidence assessment and human review remain separate.")).toBeVisible();
  await expect(page.getByText("Not Triggered",{exact:true})).toBeVisible();
  await expect(page.getByText("Not Assessed",{exact:true})).toBeVisible();
  await expect(page.getByText("Missing",{exact:true})).toBeVisible();
});

test("guidance quality uses the full variant denominator and bounded pages",async({page})=>{
  await page.route("**/api/ui/v1/**",route=>{
    const url=new URL(route.request().url());
    if(url.pathname.endsWith("bootstrap"))return route.fulfill({json:{services:[{id:"fixture",name:"Coverage fixture"}],csrf:"test",guidance_enabled:true,capabilities:{fixture:["read"]}}});
    if(url.pathname.endsWith("coverage")){
      expect(url.searchParams.get("limit")).toBe("25");
      const offset=Number(url.searchParams.get("offset"));
      return route.fulfill({json:{service_id:"fixture",catalog_id:"catalog",release_revision:0,release_hash:"",count_unit:"catalog requirement variants",variant_count:1340,counts:{missing:1340,draft:0,reviewed:0,needs_revalidation:0},complete:false,entries:[{variant_id:`variant-${offset}`,requirement_id:`RULE-${offset}`,applicability:"unresolved",state:"missing",reason:"No playbook covers this catalog variant."}],offset,next_offset:offset+25}});
    }
    return route.fulfill({json:{service_id:"fixture",binding:{},state:"missing",total:0,question_count:0,capabilities:[],questions:[],offset:0}});
  });
  await page.goto("/services/fixture?page=Readiness");
  await page.getByText("Published guidance coverage",{exact:true}).click();
  await expect(page.getByText("1340 catalog requirement variants · Coverage gate not met")).toBeVisible();
  await expect(page.getByText("Missing: 1340",{exact:true})).toBeVisible();
  await expect(page.getByText("RULE-0",{exact:true})).toBeVisible();
  await page.getByRole("button",{name:"Next variants"}).click();
  await expect(page.getByText("RULE-25",{exact:true})).toBeVisible();
  await expect(page.getByText("RULE-0",{exact:true})).toHaveCount(0);
});

test("readiness declarations preserve edits on conflict and remain distinct from proof", async ({page}) => {
  let revision=1;
  let writes=0;
  let previousKey="";
  const binding={catalog_id:"catalog",release_revision:1,release_hash:"release",profile_revision:1,scope_revision:1};
  const scope={kind:"service"};
  const data={capability_id:"monitoring",scope,owner_role:"Security operations",implementation:"unknown",trigger:"unknown",decisions:{},evidence:[],notes:"Initial note"};
  await page.route("**/api/ui/v1/**",async route=>{
    if(route.request().url().endsWith("bootstrap"))return route.fulfill({json:{services:[{id:"fixture",name:"Readiness fixture"}],csrf:"test",guidance_enabled:true,capabilities:{fixture:["read","edit"]}}});
    if(route.request().method()==="POST"){
      const body=route.request().postDataJSON();
      expect(route.request().headers()["x-csrf-token"]).toBe("test");
      expect(body.idempotency_key).toMatch(/^[a-f0-9]{32}$/);
      expect(body.idempotency_key).not.toBe(previousKey);
      previousKey=body.idempotency_key;
      writes++;
      if(writes===1){revision=2;return route.fulfill({status:409,json:{code:"revision_conflict",title:"Changes were saved elsewhere",explanation:"Compare current declarations before retrying."}});}
      expect(body.expected_revision).toBe(2);
      expect(body.data.notes).toBe("My retained note");
      expect(body.data.implementation).toBe("declared_present");
      revision=3;
      return route.fulfill({json:{revision}});
    }
    return route.fulfill({json:{service_id:"fixture",binding,state:"reviewed",total:1,count_unit:"scoped capability instances",offset:0,question_count:0,questions:[],capabilities:[{
      id:"instance",capability_id:"monitoring",name:"Monitoring",scope,requirement_ids:["RULE-ONE","RULE-TWO"],prerequisite_ids:[],implementation:revision===3?"declared_present":"unknown",evidence_state:"missing",trigger:"unknown",review:"not_assessed",blocked:false,needs_revalidation:false,steps:{items:[],none_reason:"Existing setup can be reused when its scope is demonstrated."},alternatives:[],next_action:"Collect operating records.",rationale:"A declaration is not operating proof.",declaration:{id:"instance",service_id:"fixture",revision,binding,data:{...data,notes:revision===2?"Someone else's note":"Initial note"}},
    }]}});
  });
  await page.goto("/services/fixture?page=Readiness");
  await expect(page.getByText("1 scoped capability instances · 0 unresolved questions")).toBeVisible();
  await expect(page.getByText("Missing",{exact:true})).toBeVisible();
  await page.getByText("Record ownership or implementation declaration").click();
  await page.getByLabel("Notes",{exact:true}).fill("My retained note");
  await page.getByLabel("Implementation declaration").selectOption("declared_present");
  await page.getByRole("button",{name:"Save declaration",exact:true}).click();
  await expect(page.getByText("Changes were saved elsewhere")).toBeVisible();
  await expect(page.getByLabel("Notes",{exact:true})).toHaveValue("My retained note");
  await page.getByRole("button",{name:"Load current declaration for comparison"}).click();
  await expect(page.locator("pre")).toContainText("Someone else's note");
  await page.getByRole("button",{name:"I compared the current declaration; keep my edits"}).click();
  await page.getByRole("button",{name:"Save declaration",exact:true}).click();
  await expect(page.getByText("Declaration saved. Evidence assessment and human review remain separate.")).toBeVisible();
  await expect(page.getByText("Missing",{exact:true})).toBeVisible();
  await expect(page.getByText("Not Assessed",{exact:true})).toBeVisible();
});
