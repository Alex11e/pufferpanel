import defaultRoute from './defaultRoute'

export default (api) => [
  {
    path: '/',
    redirect: () => defaultRoute(api)
  },
  {
    path: '/auth/login',
    component: () => import('@/views/Login.vue'),
    name: 'Login',
    meta: {
      noAuth: true
    }
  },
  {
    path: '/auth/register',
    component: () => import('@/views/Registration.vue'),
    name: 'Register',
    meta: {
      noAuth: true
    }
  },
  {
    path: '/auth/invite',
    component: () => import('@/views/Invite.vue'),
    name: 'Invite',
    meta: {
      noAuth: true
    }
  },
  {
    path: '/servers',
    component: () => import('@/views/ServerList.vue'),
    name: 'ServerList',
    meta: {
      tkey: 'servers.Servers',
      permission: true,
      icon: 'server',
      hotkey: 'g s'
    }
  },
  {
    path: '/servers/new',
    component: () => import('@/views/ServerCreate.vue'),
    name: 'ServerCreate'
  },
  {
    path: '/vps',
    component: () => import('@/views/VpsList.vue'),
    name: 'VpsList',
    meta: {
      tkey: 'VPS',
      permission: true,
      icon: 'server',
      hotkey: 'g v'
    }
  },
  {
    path: '/vps/new',
    component: () => import('@/views/VpsCreate.vue'),
    name: 'VpsCreate'
  },
  {
    path: '/contabo',
    component: () => import('@/views/ContaboVps.vue'),
    name: 'ContaboVps',
    meta: {
      tkey: 'Contabo VPS',
      permission: 'admin',
      icon: 'node'
    }
  },
  {
    path: '/hosting/web',
    component: () => import('@/views/VpsList.vue'),
    name: 'WebHostingList',
    props: { type: 'webhosting', title: 'Webtárhely', hint: 'PHP + Apache tárhelyek saját fájlkezelővel és SFTP-vel.', createRoute: 'WebHostingCreate', empty: 'Még nincs webtárhely.' },
    meta: {
      tkey: 'Webtárhely',
      permission: true,
      icon: 'server',
      hotkey: 'g w'
    }
  },
  {
    path: '/hosting/web/new',
    component: () => import('@/views/HostingCreate.vue'),
    name: 'WebHostingCreate',
    props: { kind: 'web' }
  },
  {
    path: '/hosting/db',
    component: () => import('@/views/VpsList.vue'),
    name: 'DbHostingList',
    props: { type: 'dbhosting', title: 'Adatbázisok', hint: 'MariaDB és PostgreSQL adatbázis-szerverek.', createRoute: 'DbHostingCreate', empty: 'Még nincs adatbázis.' },
    meta: {
      tkey: 'Adatbázisok',
      permission: true,
      icon: 'server',
      hotkey: 'g b'
    }
  },
  {
    path: '/hosting/db/new',
    component: () => import('@/views/HostingCreate.vue'),
    name: 'DbHostingCreate',
    props: { kind: 'db' }
  },
  {
    path: '/hosting/discord-bot',
    component: () => import('@/views/VpsList.vue'),
    name: 'DiscordBotList',
    props: { type: 'discordbot', title: 'Discord bot', hint: 'Discord botok Node.js futtatókörnyezettel.', createRoute: 'DiscordBotCreate', empty: 'Még nincs Discord bot.' },
    meta: {
      tkey: 'Discord bot',
      permission: true,
      icon: 'server',
      hotkey: 'g d b'
    }
  },
  {
    path: '/hosting/discord-bot/new',
    component: () => import('@/views/HostingCreate.vue'),
    name: 'DiscordBotCreate',
    props: { kind: 'discordbot' }
  },
  {
    path: '/dashboard',
    component: () => import('@/views/Dashboard.vue'),
    name: 'Dashboard',
    meta: {
      tkey: 'Kezdőlap',
      permission: true,
      icon: 'home',
      hotkey: 'g d'
    }
  },
  {
    path: '/admin',
    component: () => import('@/views/AdminDashboard.vue'),
    name: 'AdminDashboard',
    meta: {
      tkey: 'Admin központ',
      permission: 'admin',
      icon: 'admin',
      hotkey: 'g a'
    }
  },
  {
    path: '/support',
    component: () => import('@/views/SupportTickets.vue'),
    name: 'SupportTickets',
    meta: {
      tkey: 'Támogatás',
      permission: true,
      icon: 'help',
      hotkey: 'g h'
    }
  },
  {
    path: '/servers/view/:id',
    component: () => import('@/views/ServerView.vue'),
    name: 'ServerView'
  },
  {
    path: '/nodes',
    component: () => import('@/views/NodeList.vue'),
    name: 'NodeList',
    meta: {
      tkey: 'nodes.Nodes',
      permission: 'nodes.view',
      icon: 'node',
      hotkey: 'g n'
    }
  },
  {
    path: '/nodes/new',
    component: () => import('@/views/NodeCreate.vue'),
    name: 'NodeCreate'
  },
  {
    path: '/nodes/view/:id',
    component: () => import('@/views/NodeView.vue'),
    name: 'NodeView'
  },
  {
    path: '/users',
    component: () => import('@/views/UserList.vue'),
    name: 'UserList',
    meta: {
      tkey: 'users.Users',
      permission: 'users.info.view',
      icon: 'users',
      hotkey: 'g u'
    }
  },
  {
    path: '/users/new',
    component: () => import('@/views/UserCreate.vue'),
    name: 'UserCreate'
  },
  {
    path: '/users/view/:id',
    component: () => import('@/views/UserView.vue'),
    name: 'UserView'
  },
  {
    path: '/templates',
    component: () => import('@/views/TemplateList.vue'),
    name: 'TemplateList',
    meta: {
      tkey: 'templates.Templates',
      permission: 'templates.view',
      icon: 'template',
      hotkey: 'g t'
    }
  },
  {
    path: '/templates/new',
    component: () => import('@/views/TemplateCreate.vue'),
    name: 'TemplateCreate'
  },
  {
    path: '/templates/view/:repo/:id',
    component: () => import('@/views/TemplateView.vue'),
    name: 'TemplateView'
  },
  {
    path: '/settings',
    component: () => import('@/views/Settings.vue'),
    name: 'Settings',
    meta: {
      tkey: 'settings.Settings',
      permission: 'settings.edit',
      icon: 'settings',
      hotkey: 'g c'
    }
  },
  {
    path: '/self',
    component: () => import('@/views/Self.vue'),
    name: 'Self'
  }
]
