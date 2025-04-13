// @ts-ignore
import React from 'react';
// @ts-ignore
import Footer from '@theme-original/DocItem/Footer';
// @ts-ignore
import type FooterType from '@theme/DocItem/Footer';
import type { WrapperProps } from '@docusaurus/types';

type Props = WrapperProps<typeof FooterType>;

export default function FooterWrapper(props: Props): JSX.Element {
  return (
    <>
        <Footer {...props} />
        <div className="card" style={{display: 'block', marginTop: '40px', textAlign: 'center', padding: '16px', border: '1px solid var(--ifm-color-emphasis-300)'}}>
            Looking for 24/7 support from the Coroot team? Subscribe to Coroot Enterprise:
            <a href="https://coroot.com/account" target="_blank" className="primary-button" style={{marginLeft: '4px'}}>
                Start free trial
            </a>
        </div>
    </>
  );
}
