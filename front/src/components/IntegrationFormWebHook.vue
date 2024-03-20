<template>
    <div>
        <div class="subtitle-1">Webhook URL</div>
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-text-field v-model="form.webhook_url" outlined dense :rules="[$validators.notEmpty]" />

        <!-- eslint-disable-next-line vue/no-mutating-props -->

        <div class="subtitle-1">Incident template</div>
        <div class="caption">
            <pre>
Available values:
            Status <b>string</b>
            URL <b>string</b>
            App <b>struct</b>{
                Name <b>string</b>
                Kind <b>string</b>
                Namespace <b>string</b>
            }
            Reports <b>map</b> of{
                Name <b>string</b>
                Check <b>string</b>
                Message <b>string</b>
                Timestamp <b>timeseries.Time</b>
            }
        </pre
            >
        </div>
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-textarea v-model="form.incident_template" outlined dense :rules="[$validators.notEmpty]" />

        <div class="subtitle-1">Notify of</div>
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-checkbox v-model="form.incidents" label="Incidents" dense />

        <div class="subtitle-1">Deployment template</div>
        <div class="caption">
            <pre>
Available values:
            Status:  <b>string</b>
            Title:   <b>string</b>
            Summary: <b>[]string</b>
            URL:     <b>string</b>
        </pre
            >
        </div>
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-textarea v-model="form.deployment_template" outlined dense :rules="[$validators.notEmpty]" />

        <div class="subtitle-1">Notify of</div>
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-checkbox v-model="form.deployments" label="Deployments" dense hide-details />
    </div>
</template>

<script>
export default {
    props: {
        form: Object,
    },
};
</script>

<style scoped></style>
