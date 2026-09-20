import { createApp } from "vue";
import { createPinia } from "pinia";
import { WML } from "@wailsio/runtime";
import { ActionBar, ActionBarButton, ActionBarIcon, Badge, Button, Card, Cascader, Cell, CellGroup, Checkbox, CouponCell, CouponList, Empty, Field, Form, Grid, GridItem, Loading, NavBar, Popup, Search, Sidebar, SidebarItem, Stepper, Swipe, SwipeCell, SwipeItem, Tab, Tabbar, TabbarItem, Tabs } from "vant";
import "vant/lib/index.css";
import App from "./App.vue";
import router from "./router";

WML.Enable();
const app = createApp(App);
app.use(createPinia());
app.use(router);
app.use(ActionBar).use(ActionBarButton).use(ActionBarIcon).use(Badge).use(Button).use(Card).use(Cascader).use(Cell).use(CellGroup).use(Checkbox).use(CouponCell).use(CouponList).use(Empty).use(Field).use(Form).use(Grid).use(GridItem).use(Loading).use(NavBar).use(Popup).use(Search).use(Sidebar).use(SidebarItem).use(Stepper).use(Swipe).use(SwipeCell).use(SwipeItem).use(Tab).use(Tabbar).use(TabbarItem).use(Tabs);
app.mount("#app");

