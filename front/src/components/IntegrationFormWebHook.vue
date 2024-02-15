<template>
    <div>
        <div class="subtitle-1">Webhook URL</div>
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-text-field v-model="form.webhook_url" outlined dense :rules="[$validators.notEmpty]" />

        <div class="subtitle-1">Correct response JSON</div>
        <div class="caption">The webhook will be considered successful if every field and value that is below is also in the response.</div>
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-text-field v-model="form.correct_json" outlined dense :rules="[$validators.notEmpty]" />

        <div class="subtitle-1">Incident template</div>
        <div class="caption"><pre>Available values:
            StatusOK, StatusINFO, StatusWARNING, StatusCRITICAL <b>bool</b>
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
        </pre>
        </div>
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-textarea v-model="form.incident_template" outlined dense :rules="[$validators.notEmpty]" />

        <div class="subtitle-1">Notify of</div>
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-checkbox v-model="form.incidents" label="Incidents" dense hide-details />
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
