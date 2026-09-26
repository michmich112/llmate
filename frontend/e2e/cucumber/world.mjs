import { World, setWorldConstructor } from '@cucumber/cucumber';

export class LlmateWorld extends World {
  constructor(attrs) {
    super(attrs);
    this.baseURL = 'http://127.0.0.1:8099';
    this.authKey = 'e2e-key';
    this.browser = null;
    this.context = null;
    this.page = null;
    this.consoleErrors = [];
    this.providerId = '';
    this.modelRecordId = '';
  }

  async screenshot(name) {
    const buf = await this.page.screenshot({ fullPage: true });
    await this.attach(buf, { mediaType: 'image/png', fileName: `${name}.png` });
  }
}

setWorldConstructor(LlmateWorld);
