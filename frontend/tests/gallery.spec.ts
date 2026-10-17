import { expect,test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
test("progress links keep filters and evidence stays paginated",async({page})=>{
  await page.goto("/gallery");
  await expect(page.getByRole("heading",{name:"See where you stand."})).toBeVisible();
  await page.getByRole("link",{name:/Passing requirements/}).click();
  await expect(page).toHaveURL(/status=passing/);
  await expect(page.getByRole("heading",{name:"No matching requirements"})).toBeVisible();
  await page.getByRole("button",{name:"Clear filter"}).click();
  await expect(page.locator("article")).toHaveCount(4);
  await page.goto("/gallery?page=Evidence");
  await expect(page.locator("article")).toHaveCount(25);
  await page.getByRole("button",{name:"Next"}).click();
  await expect(page.getByText("Page 2 of 12")).toBeVisible();
});
test("gallery fits viewport",async({page},info)=>{
  for(const route of ["Overview","Requirements","Evidence","Sources"]){
    await page.goto(`/gallery?page=${route}`);
    await expect(page.locator("h1")).toBeVisible();
    const audit=await new AxeBuilder({page}).withTags(["wcag2a","wcag2aa","wcag21aa","wcag22aa"]).analyze();
    expect(audit.violations).toEqual([]);
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
    await page.screenshot({path:info.outputPath(`${route}.png`),fullPage:true});
  }
});
test("keyboard navigation drawer restores focus",async({page},info)=>{
 await page.goto("/gallery");
 await page.keyboard.press("Tab");await expect(page.getByRole("link",{name:"Skip to content"})).toBeFocused();
 if(info.project.name!=="desktop"){
  const trigger=page.getByRole("button",{name:"Open navigation"});await page.keyboard.press("Tab");await expect(trigger).toBeFocused();await page.keyboard.press("Enter");await expect(page.getByRole("dialog")).toBeVisible();await page.keyboard.press("Escape");await expect(trigger).toBeFocused();
 }
});
