module.exports = {
 testDir: './tests',
 timeout: 30000,
 workers: 1,
 reporter: 'list',
 use: { headless: true, ...(process.env.SC_HTTP_USERNAME ? {httpCredentials: {username: process.env.SC_HTTP_USERNAME, password: process.env.SC_HTTP_PASSWORD}} : {}) },
 outputDir: 'test-results'
};
