const { test, expect } = require('@playwright/test');

test.describe('Kafka UI Verification', () => {
  test('Check Kafka UI dashboard and cluster status', async ({ page }) => {
    console.log('Navigating to Kafka UI...');
    
    // Navigate to Kafka UI
    await page.goto('http://localhost:8080');
    
    // Wait for the page to load
    await page.waitForLoadState('networkidle');
    
    // Take screenshot of main dashboard
    await page.screenshot({ 
      path: 'kafka-ui-dashboard.png', 
      fullPage: true 
    });
    
    console.log('Dashboard screenshot taken');
    
    // Check for cluster status indicators
    const clusterElements = await page.locator('[data-testid*="cluster"], [class*="cluster"], h1, h2, h3').all();
    
    console.log('Looking for cluster status...');
    
    // Try to find cluster status text
    const pageText = await page.textContent('body');
    console.log('Page contains cluster information:', pageText.includes('cluster'));
    
    if (pageText.toLowerCase().includes('online')) {
      console.log('Found "online" status on page');
    }
    
    if (pageText.toLowerCase().includes('offline')) {
      console.log('Found "offline" status on page');
    }
    
    // Look for navigation links or cluster details
    const navLinks = await page.locator('nav a, .nav a, [role="navigation"] a').all();
    console.log(`Found ${navLinks.length} navigation links`);
    
    // Try to find and click on cluster details if available
    const clusterLink = page.locator('text=/cluster/i').first();
    const clusterLinkExists = await clusterLink.count() > 0;
    
    if (clusterLinkExists) {
      console.log('Found cluster link, attempting to navigate...');
      await clusterLink.click();
      await page.waitForLoadState('networkidle');
      
      // Take screenshot of cluster details
      await page.screenshot({ 
        path: 'kafka-cluster-details.png', 
        fullPage: true 
      });
      
      console.log('Cluster details screenshot taken');
    }
    
    // Look for topics section
    const topicsSection = page.locator('text=/topics?/i').first();
    const topicsExists = await topicsSection.count() > 0;
    
    if (topicsExists) {
      console.log('Found topics section, taking screenshot...');
      await topicsSection.click();
      await page.waitForLoadState('networkidle');
      
      await page.screenshot({ 
        path: 'kafka-topics.png', 
        fullPage: true 
      });
      
      console.log('Topics screenshot taken');
    }
    
    // Check for brokers information
    const brokersSection = page.locator('text=/brokers?/i').first();
    const brokersExists = await brokersSection.count() > 0;
    
    if (brokersExists) {
      console.log('Found brokers section, taking screenshot...');
      await brokersSection.click();
      await page.waitForLoadState('networkidle');
      
      await page.screenshot({ 
        path: 'kafka-brokers.png', 
        fullPage: true 
      });
      
      console.log('Brokers screenshot taken');
    }
    
    // Return to main dashboard for final screenshot
    await page.goto('http://localhost:8080');
    await page.waitForLoadState('networkidle');
    
    // Take final screenshot
    await page.screenshot({ 
      path: 'kafka-ui-final.png', 
      fullPage: true 
    });
    
    console.log('All screenshots completed');
  });
  
  test('Extract cluster and connection information', async ({ page }) => {
    await page.goto('http://localhost:8080');
    await page.waitForLoadState('networkidle');
    
    // Extract all text content for analysis
    const bodyText = await page.textContent('body');
    console.log('=== KAFKA UI PAGE CONTENT ===');
    console.log(bodyText);
    console.log('=== END PAGE CONTENT ===');
    
    // Look for specific status indicators
    const statusElements = await page.locator('[class*="status"], [class*="health"], [data-testid*="status"]').all();
    
    for (let i = 0; i < statusElements.length; i++) {
      const element = statusElements[i];
      const text = await element.textContent();
      console.log(`Status element ${i + 1}: ${text}`);
    }
    
    // Check for error messages
    const errorElements = await page.locator('[class*="error"], [class*="warning"], .alert').all();
    
    for (let i = 0; i < errorElements.length; i++) {
      const element = errorElements[i];
      const text = await element.textContent();
      console.log(`Error/Warning element ${i + 1}: ${text}`);
    }
  });
  
  test('Verify eru-test-topic and messages', async ({ page }) => {
    console.log('Checking for eru-test-topic and messages...');
    
    await page.goto('http://localhost:8080');
    await page.waitForLoadState('networkidle');
    
    // Navigate to Topics section
    const topicsLink = page.locator('text=/topics/i').first();
    const topicsExists = await topicsLink.count() > 0;
    
    if (topicsExists) {
      console.log('Navigating to Topics section...');
      await topicsLink.click();
      await page.waitForLoadState('networkidle');
      
      // Look for eru-test-topic
      const pageText = await page.textContent('body');
      console.log('Checking for eru-test-topic...');
      
      if (pageText.includes('eru-test-topic')) {
        console.log('✓ Found eru-test-topic in topics list');
        
        // Try to click on the topic for details
        const topicLink = page.locator('text=eru-test-topic').first();
        const topicLinkExists = await topicLink.count() > 0;
        
        if (topicLinkExists) {
          console.log('Clicking on eru-test-topic for details...');
          await topicLink.click();
          await page.waitForLoadState('networkidle');
          
          // Take screenshot of topic details
          await page.screenshot({ 
            path: 'eru-test-topic-details.png', 
            fullPage: true 
          });
          
          // Check for messages or statistics
          const topicPageText = await page.textContent('body');
          console.log('Topic page content includes messages:', topicPageText.includes('message'));
          console.log('Topic page content includes records:', topicPageText.includes('record'));
          
          if (topicPageText.includes('Hello from Eru Events')) {
            console.log('✓ Found our test message content!');
          }
        }
      } else {
        console.log('✗ eru-test-topic not found in topics list');
      }
      
      // Take screenshot of topics overview
      await page.screenshot({ 
        path: 'kafka-topics-overview.png', 
        fullPage: true 
      });
    }
    
    // Take final comprehensive screenshot
    await page.goto('http://localhost:8080');
    await page.waitForLoadState('networkidle');
    
    await page.screenshot({ 
      path: 'kafka-ui-verification-final.png', 
      fullPage: true 
    });
    
    console.log('Topic verification completed');
  });
});