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
    props: { type: 'discordbot', title: 'Discord bot', hint: 'Discord botok Node.js vagy Python futtatókörnyezettel.', createRoute: 'DiscordBotCreate', empty: 'Még nincs Discord bot.' },
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
    path: '/jellyfin',
    component: () => import('@/views/VpsList.vue'),
    name: 'JellyfinList',
    props: { type: 'jellyfin', titleKey: 'servers.services.jellyfin.title', hintKey: 'servers.services.jellyfin.hint', createRoute: 'JellyfinCreate', emptyKey: 'servers.services.jellyfin.empty' },
    meta: { tkey: 'servers.services.jellyfin.title', permission: true, icon: 'server', hotkey: 'g j' }
  },
  {
    path: '/jellyfin/new',
    component: () => import('@/views/HostingCreate.vue'),
    name: 'JellyfinCreate',
    props: { kind: 'jellyfin' }
  },
  {
    path: '/staticduomusic',
    component: () => import('@/views/VpsList.vue'),
    name: 'StaticduomusicList',
    props: { type: 'staticduomusic', titleKey: 'servers.services.staticduomusic.title', hintKey: 'servers.services.staticduomusic.hint', createRoute: 'StaticduomusicCreate', emptyKey: 'servers.services.staticduomusic.empty' },
    meta: { tkey: 'servers.services.staticduomusic.title', permission: true, icon: 'server', hotkey: 'g m' }
  },
  {
    path: '/staticduomusic/new',
    component: () => import('@/views/HostingCreate.vue'),
    name: 'StaticduomusicCreate',
    props: { kind: 'staticduomusic' }
  },
  {
    path: '/storage',
    component: () => import('@/views/VpsList.vue'),
    name: 'StorageList',
    props: { type: 'storage', titleKey: 'servers.services.storage.title', hintKey: 'servers.services.storage.hint', createRoute: 'StorageCreate', emptyKey: 'servers.services.storage.empty' },
    meta: { tkey: 'servers.services.storage.title', permission: true, icon: 'files', hotkey: 'g s t' }
  },
  {
    path: '/storage/new',
    component: () => import('@/views/HostingCreate.vue'),
    name: 'StorageCreate',
    props: { kind: 'storage' }
  },
  {
    path: '/gibbon',
    component: () => import('@/views/VpsList.vue'),
    name: 'GibbonList',
    props: { type: 'gibbon', titleKey: 'servers.services.gibbon.title', hintKey: 'servers.services.gibbon.hint', createRoute: 'GibbonCreate', emptyKey: 'servers.services.gibbon.empty' },
    meta: { tkey: 'servers.services.gibbon.title', permission: true, icon: 'school', hotkey: 'g e' }
  },
  {
    path: '/gibbon/new',
    component: () => import('@/views/HostingCreate.vue'),
    name: 'GibbonCreate',
    props: { kind: 'gibbon' }
  },
  {
    path: '/storage/smb',
    component: () => import('@/views/VpsList.vue'),
    name: 'SambaStorageList',
    props: { type: 'samba', titleKey: 'servers.services.smb.title', hintKey: 'servers.services.smb.hint', createRoute: 'SambaStorageCreate', emptyKey: 'servers.services.smb.empty' },
    meta: { tkey: 'servers.services.smb.title', permission: 'admin', icon: 'server' }
  },
  {
    path: '/storage/smb/new',
    component: () => import('@/views/HostingCreate.vue'),
    name: 'SambaStorageCreate',
    props: { kind: 'samba' }
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
    path: '/admin/social-login',
    component: () => import('@/views/SocialLogin.vue'),
    name: 'SocialLoginAdmin',
    meta: {
      tkey: 'Külső bejelentkezések',
      permission: 'admin',
      icon: 'account'
    }
  },
  {
    path: '/admin/billing',
    component: () => import('@/views/BillingPlans.vue'),
    name: 'BillingPlans',
    meta: {
      tkey: 'Csomagok',
      permission: 'admin',
      icon: 'admin'
    }
  },
  {
    path: '/billing/checkout',
    component: () => import('@/views/BillingCheckout.vue'),
    name: 'BillingCheckout'
  },
  {
    path: '/billing/complete',
    component: () => import('@/views/BillingComplete.vue'),
    name: 'BillingComplete'
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
