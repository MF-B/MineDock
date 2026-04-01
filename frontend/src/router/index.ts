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
  ],
});

export default router;
