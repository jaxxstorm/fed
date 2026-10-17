import {expect,test} from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
const service="dc0c7bef-7a80-4233-9ff2-cb69c6b0554f";
test("live assessment pages and deep links",async({page},info)=>{
 test.setTimeout(120000);
 const errors:string[]=[];page.on("pageerror",e=>errors.push(e.message));
 await page.goto("/");await expect(page.getByRole("heading",{name:"Your services"})).toBeVisible();await page.screenshot({path:info.outputPath("Services.png")});
 for(const route of ["Overview","Requirements","Evidence","Activity","Sources","Settings","Exports"]){
  await page.goto(`/services/${service}?page=${route}`);
  await expect(page.locator("h1").first()).toBeVisible({timeout:60000});
  await expect(page.getByRole("alert")).toHaveCount(0);
  const audit=await new AxeBuilder({page}).withTags(["wcag2a","wcag2aa","wcag21aa","wcag22aa"]).analyze();
  expect(audit.violations).toEqual([]);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await page.screenshot({path:info.outputPath(`${route}.png`),fullPage:false});
 }
 await page.goto(`/services/${service}/requirements/KSI-SVC-ACM`);
 await expect(page.getByRole("heading",{name:/KSI-SVC-ACM/})).toBeVisible({timeout:60000});
 await expect(page.getByRole("heading",{name:"Supporting evidence"})).toBeVisible();
 expect((await new AxeBuilder({page}).withTags(["wcag2a","wcag2aa","wcag21aa","wcag22aa"]).analyze()).violations).toEqual([]);
 await page.screenshot({path:info.outputPath("Requirement.png")});
 await page.getByRole("button",{name:"Inspect evidence"}).first().click();
 await expect(page.getByRole("dialog")).toBeVisible();
 await expect(page.locator(".artifact-reader")).toBeVisible();
 expect((await new AxeBuilder({page}).withTags(["wcag2a","wcag2aa","wcag21aa","wcag22aa"]).analyze()).violations).toEqual([]);
 await page.screenshot({path:info.outputPath("Evidence-preview.png"),fullPage:false});
 await page.getByRole("button",{name:"Close evidence"}).click();
 const run=page.locator('a[href*="/runs/"]').first();
 if(await run.count()){await run.click();await expect(page.getByRole("heading",{name:"Collection run",exact:true})).toBeVisible();await page.screenshot({path:info.outputPath("Run.png")});}
 expect(errors).toEqual([]);
});
test("live progress counts reconcile with drill-downs",async({page})=>{
 const response=page.waitForResponse(r=>r.url().includes("/overview")&&r.status()===200);
 await page.goto(`/services/${service}`);
 const model=await (await response).json();
 await page.getByRole("link",{name:/Passing requirements/}).click();
 await expect(page.getByRole("heading",{name:`${model.counts.passing} requirements`,exact:true})).toBeVisible();
 await page.goto(`/services/${service}?page=Requirements&applicability=required&status=failing`);
 await expect(page.getByRole("heading",{name:`${model.counts.statuses.failing||0} requirements`,exact:true})).toBeVisible();
});
