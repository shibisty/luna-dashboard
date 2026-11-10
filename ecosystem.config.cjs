module.exports = {
  apps: [
    {
      name: 'luna-dashboard',
      script: '.output/server/index.mjs',
      instances: 1,          // можно 'max' для кластер-режима на все ядра
      exec_mode: 'fork',     // 'cluster' — если instances > 1
      env: {
        NODE_ENV: 'production',
        PORT: 3000,
        HOST: '127.0.0.1',
      },
      max_memory_restart: '300M',
      autorestart: true,
    },
  ],
}
