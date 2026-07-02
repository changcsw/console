import { describe, expect, test } from "vitest";
import { defineComponent } from "vue";
import { mount } from "@vue/test-utils";
import DashboardMetricCard from "@/views/dashboard/components/DashboardMetricCard.vue";

describe("DashboardMetricCard", () => {
  test("count=0 显示绿色『暂无待办』", () => {
    const wrapper = mount(DashboardMetricCard, {
      props: {
        title: "汇率待审",
        value: 0,
        envScoped: true
      }
    });

    expect(wrapper.text()).toContain("暂无待办");
    expect(wrapper.find(".metric-card__value--ok").exists()).toBe(true);
    expect(wrapper.find(".metric-card__value--warning").exists()).toBe(false);
  });

  test("count>0 显示警示色；envScoped=false 显示『全环境』角标", () => {
    const wrapper = mount(DashboardMetricCard, {
      props: {
        title: "汇率待审",
        value: 3,
        envScoped: false
      }
    });

    expect(wrapper.text()).toContain("存在待处理项");
    expect(wrapper.find(".metric-card__value--warning").exists()).toBe(true);
    expect(wrapper.text()).toContain("全环境");
  });

  test("展开明细按钮可触发事件并渲染 details 插槽", async () => {
    const wrapper = mount(DashboardMetricCard, {
      props: {
        title: "最近同步",
        value: 1,
        envScoped: true,
        expandable: true,
        detailsExpanded: true
      },
      slots: {
        details: "<div class='test-details'>job-88</div>"
      }
    });

    const toggleBtn = wrapper.findAll("button").find((btn) => btn.text().includes("收起明细"));
    expect(toggleBtn).toBeTruthy();
    expect(wrapper.find(".test-details").exists()).toBe(true);
    await toggleBtn!.trigger("click");
    expect(wrapper.emitted("toggleDetails")).toHaveLength(1);
  });

  test("前往处理按钮触发 navigate 事件", async () => {
    const wrapper = mount(DashboardMetricCard, {
      props: {
        title: "配置异常",
        value: 2,
        envScoped: true
      }
    });

    const navBtn = wrapper.findAll("button").find((btn) => btn.text().includes("前往处理"));
    expect(navBtn).toBeTruthy();
    await navBtn!.trigger("click");
    expect(wrapper.emitted("navigate")).toHaveLength(1);
  });

  test("permitted=false 时可由父层整块隐藏", () => {
    const Host = defineComponent({
      components: { DashboardMetricCard },
      props: { permitted: { type: Boolean, required: true } },
      template:
        "<DashboardMetricCard v-if='permitted' title='配置异常' :value='1' :env-scoped='true' /><div v-else class='forbidden'>无权限</div>"
    });

    const wrapper = mount(Host, { props: { permitted: false } });
    expect(wrapper.findComponent(DashboardMetricCard).exists()).toBe(false);
    expect(wrapper.find(".forbidden").exists()).toBe(true);
  });
});
