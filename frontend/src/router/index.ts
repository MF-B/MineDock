import { createRouter, createWebHistory } from "vue-router";
import ContainerList from "../views/ContainerList.vue";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/",
      name: "ContainerList",
      component: ContainerList,
    },
    {
      path: "/registry",
      name: "ImageRegistry",
      component: () => import("../views/ImageRegistry.vue"),
    },
    {
      path: "/registry/:gameId/create",
      name: "CreateInstance",
      component: () => import("../views/CreateInstance.vue"),
    },
    {
      path: "/instances/:id",
      name: "InstanceDetail",
      component: () => import("../views/InstanceDetail.vue"),
    },
  ],
});

export default router;
