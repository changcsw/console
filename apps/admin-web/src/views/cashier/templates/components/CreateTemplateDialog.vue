<template>
  <el-dialog
    :model-value="open"
    title="新建收银台模板"
    width="560px"
    :close-on-click-modal="false"
    @update:model-value="(v: boolean) => !v && emit('close')"
  >
    <el-form :model="form" label-position="top">
      <el-form-item label="模板 ID" required>
        <el-input v-model.trim="form.templateId" placeholder="如：global_default" />
      </el-form-item>
      <el-form-item label="模板名称" required>
        <el-input v-model.trim="form.templateName" placeholder="如：Global Default" />
      </el-form-item>
      <el-form-item label="启用汇率同步">
        <el-switch v-model="form.fxSyncEnabled" />
      </el-form-item>
      <el-form-item label="汇率同步模式">
        <el-radio-group v-model="form.fxSyncMode">
          <el-radio value="manual_confirm">manual_confirm（默认人工确认）</el-radio>
          <el-radio value="auto_apply">auto_apply（自动应用）</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="同步周期">
        <el-radio-group v-model="form.fxSyncSchedule">
          <el-radio value="monthly">monthly</el-radio>
          <el-radio value="quarterly">quarterly</el-radio>
        </el-radio-group>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="emit('close')">取消</el-button>
      <el-button v-perm="'cashier.write'" type="primary" :loading="submitting" @click="submit">创建模板</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { ApiError } from "@/api/http";
import { createCashierTemplate, type FxSyncMode, type FxSyncSchedule } from "@/api/modules/cashier";

const props = defineProps<{
  open: boolean;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "created"): void;
}>();

const form = reactive<{
  templateId: string;
  templateName: string;
  fxSyncEnabled: boolean;
  fxSyncMode: FxSyncMode;
  fxSyncSchedule: FxSyncSchedule;
}>({
  templateId: "",
  templateName: "",
  fxSyncEnabled: true,
  fxSyncMode: "manual_confirm",
  fxSyncSchedule: "monthly"
});

const submitting = ref(false);

watch(
  () => props.open,
  (open) => {
    if (open) {
      form.templateId = "";
      form.templateName = "";
      form.fxSyncEnabled = true;
      form.fxSyncMode = "manual_confirm";
      form.fxSyncSchedule = "monthly";
    }
  }
);

async function submit() {
  if (!form.templateId || !form.templateName) {
    ElMessage.warning("请填写模板 ID 与模板名称");
    return;
  }
  submitting.value = true;
  try {
    await createCashierTemplate({ ...form });
    ElMessage.success("模板已创建");
    emit("created");
    emit("close");
  } catch (err) {
    if (err instanceof ApiError && err.code === "CONFLICT") {
      ElMessage.error("templateId 已存在");
    } else {
      ElMessage.error(err instanceof ApiError ? err.message : "创建模板失败");
    }
  } finally {
    submitting.value = false;
  }
}
</script>
