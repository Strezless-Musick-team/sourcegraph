import * as H from 'history'
import React, { FunctionComponent, useCallback, useEffect, useState } from 'react'
import { RouteComponentProps } from 'react-router'

import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { ThemeProps } from '@sourcegraph/shared/src/theme'
import { Container, PageHeader } from '@sourcegraph/wildcard'

import { ErrorAlert } from '../../../components/alerts'
import { PageTitle } from '../../../components/PageTitle'
import { DynamicallyImportedMonacoSettingsEditor } from '../../../settings/DynamicallyImportedMonacoSettingsEditor'

import { getConfiguration as defaultGetConfiguration, updateConfiguration } from './backend'
import allConfigSchema from './schema.json'

export interface CodeIntelIndexConfigurationPageProps extends RouteComponentProps<{}>, ThemeProps, TelemetryProps {
    repo: { id: string }
    history: H.History
    getConfiguration?: typeof defaultGetConfiguration
}

enum CodeIntelIndexEditorState {
    Idle,
    Saving,
}

export const CodeIntelIndexConfigurationPage: FunctionComponent<CodeIntelIndexConfigurationPageProps> = ({
    repo,
    isLightTheme,
    telemetryService,
    history,
    getConfiguration = defaultGetConfiguration,
}) => {
    useEffect(() => telemetryService.logViewEvent('CodeIntelIndexConfigurationPage'), [telemetryService])

    const [fetchError, setFetchError] = useState<Error>()
    const [saveError, setSaveError] = useState<Error>()
    const [state, setState] = useState(() => CodeIntelIndexEditorState.Idle)
    const [configuration, setConfiguration] = useState('')
    const [inferredConfiguration, setInferredConfiguration] = useState('')

    useEffect(() => {
        const subscription = getConfiguration({ id: repo.id }).subscribe(configuration => {
            setConfiguration(configuration?.indexConfiguration?.configuration || '')
            setInferredConfiguration(configuration?.indexConfiguration?.inferredConfiguration || '')
        }, setFetchError)

        return () => subscription.unsubscribe()
    }, [repo, getConfiguration])

    const save = useCallback(
        async (content: string) => {
            setState(CodeIntelIndexEditorState.Saving)
            setSaveError(undefined)

            try {
                await updateConfiguration({ id: repo.id, content }).toPromise()
            } catch (error) {
                setSaveError(error)
            } finally {
                setState(CodeIntelIndexEditorState.Idle)
            }
        },
        [repo]
    )

    const runInferConfiguration = useCallback(
        (config: string) => ({ edits: [{ offset: 0, length: config.length, content: inferredConfiguration }] }),
        [inferredConfiguration]
    )

    const saving = state === CodeIntelIndexEditorState.Saving

    return fetchError ? (
        <ErrorAlert prefix="Error fetching index configuration" error={fetchError} />
    ) : (
        <div className="code-intel-index-configuration">
            <PageTitle title="Precise code intelligence index configuration" />
            <PageHeader
                headingElement="h2"
                path={[
                    {
                        text: <>Precise code intelligence index configuration</>,
                    },
                ]}
                description={
                    <>
                        Override the inferred configuration when automatically indexing repositories on{' '}
                        <a href="https://sourcegraph.com" target="_blank" rel="noreferrer noopener">
                            Sourcegraph.com
                        </a>
                        .
                    </>
                }
                className="mb-3"
            />

            <Container>
                {saveError && <ErrorAlert prefix="Error saving index configuration" error={saveError} />}

                <DynamicallyImportedMonacoSettingsEditor
                    value={configuration}
                    jsonSchema={allConfigSchema}
                    canEdit={true}
                    onChange={setConfiguration}
                    onSave={save}
                    saving={saving}
                    height={600}
                    isLightTheme={isLightTheme}
                    history={history}
                    telemetryService={telemetryService}
                    actions={[
                        {
                            id: 'inferConfiguration',
                            label: 'Infer configuration from HEAD',
                            run: runInferConfiguration,
                        },
                    ]}
                />
            </Container>
        </div>
    )
}
