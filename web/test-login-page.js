import { chromium } from 'playwright';

(async () => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  
  try {
    console.log('Navigating to login page...');
    await page.goto('http://localhost:5173/login');
    
    console.log('Page title:', await page.title());
    
    console.log('\n=== Page HTML ===');
    const html = await page.content();
    console.log(html.substring(0, 2000) + '...'); // 只打印前2000个字符
    
    console.log('\n=== Input elements ===');
    const inputs = await page.$$('input');
    for (let i = 0; i < inputs.length; i++) {
      const input = inputs[i];
      const name = await input.getAttribute('name');
      const type = await input.getAttribute('type');
      const placeholder = await input.getAttribute('placeholder');
      console.log(`Input ${i}: name=${name}, type=${type}, placeholder=${placeholder}`);
    }
    
    console.log('\n=== Submit button ===');
    const buttons = await page.$$('button');
    for (let i = 0; i < buttons.length; i++) {
      const button = buttons[i];
      const text = await button.textContent();
      const type = await button.getAttribute('type');
      console.log(`Button ${i}: text=${text}, type=${type}`);
    }
    
  } catch (error) {
    console.error('Error:', error);
  } finally {
    await browser.close();
  }
})();
