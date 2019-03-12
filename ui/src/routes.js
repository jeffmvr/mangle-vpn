import Vue from 'vue'
import Router from 'vue-router'
import Profile from '@/components/user/Profile'
import Admin from '@/components/admin/Admin'
import ClientList from '@/components/admin/clients/ClientList'
import EventList from '@/components/admin/events/EventList'
import GroupCreate from '@/components/admin/groups/GroupCreate'
import GroupDetail from '@/components/admin/groups/GroupDetail'
import GroupList from '@/components/admin/groups/GroupList'
import LogList from '@/components/admin/logs/LogList'
import SettingList from '@/components/admin/settings/SettingList'
import UserCreate from '@/components/admin/users/UserCreate'
import UserDetail from '@/components/admin/users/UserDetail'
import UserList from '@/components/admin/users/UserList'

Vue.use(Router);

export default new Router({
  routes: [
    //----------------------------
    // Profile
    //----------------------------
    {
      path:'/',
      name: 'Profile',
      component: Profile,
    },

    //----------------------------
    // Admin
    //----------------------------
    {
      path: '/admin',
      name: 'Admin',
      component: Admin,
      children: [
        {
          path: '',
          name: 'AdminIndex',
          redirect: 'clients'
        },

        //----------------------------
        // Clients
        //----------------------------
        {
          path: 'clients',
          name: 'ClientList',
          component: ClientList,
        },

        //----------------------------
        // Events
        //----------------------------
        {
          path: 'events',
          name: 'EventList',
          component: EventList,
        },

        //----------------------------
        // Groups
        //----------------------------
        {
          path: 'groups',
          name: 'GroupList',
          component: GroupList,
        },
        {
          path: 'groups/new',
          name: 'GroupCreate',
          component: GroupCreate,
        },
        {
          path: 'groups/:id',
          name: 'GroupDetail',
          component: GroupDetail,
        },

        //----------------------------
        // Logs
        //----------------------------
        {
          path: 'logs',
          name: 'LogList',
          component: LogList,
        },

        //----------------------------
        // Settings
        //----------------------------
        {
          path: 'settings',
          name: 'SettingList',
          component: SettingList,
        },

        //----------------------------
        // Users
        //----------------------------
        {
          path: 'users',
          name: 'UserList',
          component: UserList,
        },
        {
          path: 'users/invite',
          name: 'UserCreate',
          component: UserCreate,
          props: true,
        },
        {
          path: 'users/:id',
          name: 'UserDetail',
          component: UserDetail,
        },
      ]
    }, // End Admin Routes
  ]
})
