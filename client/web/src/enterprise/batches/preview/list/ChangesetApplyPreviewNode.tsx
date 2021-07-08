import * as H from 'history'
import React from 'react'

import { ThemeProps } from '@sourcegraph/shared/src/theme'

import { ChangesetApplyPreviewFields } from '../../../../graphql-operations'
import { PreviewPageAuthenticatedUser } from '../BatchChangePreviewPage'

import { queryChangesetSpecFileDiffs } from './backend'
import styles from './ChangesetApplyPreviewNode.module.scss'
import { HiddenChangesetApplyPreviewNode } from './HiddenChangesetApplyPreviewNode'
import { VisibleChangesetApplyPreviewNode } from './VisibleChangesetApplyPreviewNode'

export interface ChangesetApplyPreviewNodeProps extends ThemeProps {
    node: ChangesetApplyPreviewFields
    history: H.History
    location: H.Location
    authenticatedUser: PreviewPageAuthenticatedUser

    selectionEnabled: boolean
    allSelected: boolean
    onSelection: (id: string, checked: boolean) => void

    /** Used for testing. */
    queryChangesetSpecFileDiffs?: typeof queryChangesetSpecFileDiffs
    /** Expand changeset descriptions, for testing only. */
    expandChangesetDescriptions?: boolean
}

export const ChangesetApplyPreviewNode: React.FunctionComponent<ChangesetApplyPreviewNodeProps> = ({
    node,
    history,
    location,
    authenticatedUser,
    isLightTheme,
    selectionEnabled,
    allSelected,
    onSelection,
    queryChangesetSpecFileDiffs,
    expandChangesetDescriptions,
}) => {
    if (node.__typename === 'HiddenChangesetApplyPreview') {
        return (
            <>
                <span className={styles.changesetApplyPreviewNodeSeparator} />
                <HiddenChangesetApplyPreviewNode node={node} />
            </>
        )
    }
    return (
        <>
            <span className={styles.changesetApplyPreviewNodeSeparator} />
            <VisibleChangesetApplyPreviewNode
                node={node}
                history={history}
                location={location}
                isLightTheme={isLightTheme}
                authenticatedUser={authenticatedUser}
                selectionEnabled={selectionEnabled}
                allSelected={allSelected}
                onSelection={onSelection}
                queryChangesetSpecFileDiffs={queryChangesetSpecFileDiffs}
                expandChangesetDescriptions={expandChangesetDescriptions}
            />
        </>
    )
}
