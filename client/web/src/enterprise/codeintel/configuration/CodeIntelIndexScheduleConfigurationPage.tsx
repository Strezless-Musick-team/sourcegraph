import * as H from 'history'
import React, { FunctionComponent, useEffect } from 'react'
import { RouteComponentProps } from 'react-router'

import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { ThemeProps } from '@sourcegraph/shared/src/theme'

export interface CodeIntelIndexScheduleConfigurationPageProps
    extends RouteComponentProps<{}>,
        ThemeProps,
        TelemetryProps {
    repo: { id: string }
    history: H.History
}

export const CodeIntelIndexScheduleConfigurationPage: FunctionComponent<CodeIntelIndexScheduleConfigurationPageProps> = ({
    telemetryService,
}) => {
    useEffect(() => telemetryService.logViewEvent('CodeIntelIndexScheduleConfigurationPage'), [telemetryService])

    return <h1>Empty</h1>
}
