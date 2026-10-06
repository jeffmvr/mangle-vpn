import { createRouter, createWebHashHistory } from "vue-router";

import { LOG_SOURCES } from "@/lib/logs";
import { SETTINGS_SECTIONS } from "@/lib/settings";
import { useSession } from "@/stores/session";

/**
 * Hash routing, as the application already uses.
 *
 * The Go server answers one HTML document at "/" and treats every other
 * unmatched path as a genuine 404, so the routes below stay in the
 * fragment. Moving to history routing means teaching the server a
 * catch-all, which is a change worth making deliberately rather than as a
 * side effect of the rewrite.
 */
/** The pages of a group, as they appear in its address. */
const GROUP_SECTIONS = ["details", "firewall", "members"];

const routes = [
  {
    path: "/",
    component: () => import("@/layouts/UserLayout.vue"),
    children: [
      {
        path: "",
        name: "profile",
        component: () => import("@/views/ProfileView.vue"),
        meta: { title: "My devices" },
      },
    ],
  },
  {
    path: "/admin",
    component: () => import("@/layouts/AdminLayout.vue"),
    meta: { admin: true },
    children: [
      { path: "", redirect: { name: "admin-clients" } },
      {
        path: "clients",
        name: "admin-clients",
        component: () => import("@/views/admin/ClientsView.vue"),
        meta: { title: "Connected clients" },
      },
      {
        path: "users",
        name: "admin-users",
        component: () => import("@/views/admin/UsersView.vue"),
        meta: { title: "Users" },
      },
      // A user's pages: their details, devices and activity. Links to the
      // user themselves land on their details.
      {
        path: "users/:id",
        redirect: (to) => ({ name: "admin-user", params: { id: to.params.id, section: "details" } }),
      },
      {
        path: "users/:id/:section(details|devices|activity)",
        name: "admin-user",
        component: () => import("@/views/admin/UserDetailView.vue"),
        meta: { title: "User" },
      },
      {
        path: "groups",
        name: "admin-groups",
        component: () => import("@/views/admin/GroupsView.vue"),
        meta: { title: "Groups" },
      },
      // A group's pages: its details, firewall rules and members. Links to
      // the group itself, and older ones that chose a tab with ?tab=, land
      // on the right one.
      {
        path: "groups/:id",
        redirect: (to) => ({
          name: "admin-group",
          params: { id: to.params.id, section: GROUP_SECTIONS.includes(to.query.tab) ? to.query.tab : "details" },
          query: {},
        }),
      },
      {
        path: `groups/:id/:section(${GROUP_SECTIONS.join("|")})`,
        name: "admin-group",
        component: () => import("@/views/admin/GroupDetailView.vue"),
        meta: { title: "Group" },
      },
      {
        path: "activity",
        name: "admin-activity",
        component: () => import("@/views/admin/ActivityView.vue"),
        meta: { title: "Activity" },
      },
      // The page's address before it was renamed.
      {
        path: "events",
        redirect: (to) => ({ name: "admin-activity", query: to.query }),
      },
      // Earlier addresses chose the log with ?log=; send those on.
      {
        path: "logs",
        redirect: (to) => {
          const source = LOG_SOURCES.find((item) => item.id === to.query.log) ?? LOG_SOURCES[0];
          return { name: "admin-logs", params: { source: source.slug }, query: {} };
        },
      },
      {
        path: `logs/:source(${LOG_SOURCES.map((item) => item.slug).join("|")})`,
        name: "admin-logs",
        component: () => import("@/views/admin/LogsView.vue"),
        meta: { title: "Logs", adminOnly: true },
      },
      // Earlier addresses chose the section with ?tab=; send those on.
      {
        path: "settings",
        redirect: (to) => {
          const section = SETTINGS_SECTIONS.find((item) => item.id === to.query.tab) ?? SETTINGS_SECTIONS[0];
          return { name: "admin-settings", params: { section: section.slug }, query: {} };
        },
      },
      {
        path: `settings/:section(${SETTINGS_SECTIONS.map((item) => item.slug).join("|")})`,
        name: "admin-settings",
        component: () => import("@/views/admin/SettingsView.vue"),
        meta: { title: "Settings", adminOnly: true },
      },
    ],
  },
  { path: "/:pathMatch(.*)*", redirect: "/" },
];

const router = createRouter({
  history: createWebHashHistory(),
  routes,
  scrollBehavior: () => ({ top: 0 }),
});

// The server refuses every request beyond someone's role anyway; this only
// saves them from landing on a screen that cannot load. The help desk
// shares the administration pages apart from logs and settings.
router.beforeEach(async (to) => {
  if (!to.matched.some((record) => record.meta.admin)) return true;

  const session = useSession();
  if (!session.ready) await session.start();
  if (!session.canAdminister) return "/";
  if (to.meta.adminOnly && !session.isAdmin) return { name: "admin-clients" };
  return true;
});

router.afterEach((to) => {
  document.title = to.meta.title ? `${to.meta.title} · Mangle VPN` : "Mangle VPN";
});

export default router;
